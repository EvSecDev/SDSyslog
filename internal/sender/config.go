package sender

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/metrics/server"
	"sdsyslog/internal/parsing"
	"sdsyslog/pkg/crypto/registry"
	"sdsyslog/pkg/protocol"
	"slices"
	"sort"
	"strings"
	"time"
)

// Loads JSON config from file
func (daemon *Daemon) LoadConfig(path string) (err error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("failed to read config file: %w", err)
		return
	}
	daemon.configPath = path

	err = json.Unmarshal(configFile, &daemon.opts)
	if err != nil {
		err = fmt.Errorf("invalid config syntax in '%s': %w", path, err)
		return
	}

	return
}

// Retrieves private key from disk from configured path
func (daemon *Daemon) LoadPubKey() (key []byte, err error) {
	key, err = base64.StdEncoding.DecodeString(daemon.opts.PublicKey)
	if err != nil {
		err = fmt.Errorf("failed decoding public key: %w", err)
		return
	}
	return
}

// Loads all input configurations.
// Checks for input include directive and loads associated files
func (opts *JSONOptions) loadInputs() (err error) {
	if opts.Inputs.Include == "" {
		// No-op
		return
	}

	info, err := os.Stat(opts.Inputs.Include)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No-op
			err = nil
			return
		}
		return
	}

	switch mode := info.Mode(); {
	case mode.IsRegular():
		name := filepath.Base(opts.Inputs.Include)
		if strings.HasPrefix(name, ".") {
			err = fmt.Errorf("input include file %q is hidden and will not be loaded", opts.Inputs.Include)
			return
		}
		if filepath.Ext(name) != ".json" {
			err = fmt.Errorf("input include file %q does not have .json extension and will not be loaded", opts.Inputs.Include)
			return
		}

		err = opts.Inputs.mergeConfigInputs(opts.Inputs.Include)
		if err != nil {
			err = fmt.Errorf("failed loading input include %q: %w", opts.Inputs.Include, err)
			return
		}
	case mode.IsDir():
		var entries []os.DirEntry
		entries, err = os.ReadDir(opts.Inputs.Include)
		if err != nil {
			err = fmt.Errorf("failed to read directory: %w", err)
			return
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()

			// skip hidden files
			if strings.HasPrefix(name, ".") {
				continue
			}

			// Only load JSON files
			if filepath.Ext(name) != ".json" {
				continue
			}

			fullPath := filepath.Join(opts.Inputs.Include, name)
			err = opts.Inputs.mergeConfigInputs(fullPath)
			if err != nil {
				err = fmt.Errorf("failed loading input include %q: %w", fullPath, err)
				return
			}
		}
	default:
		err = fmt.Errorf("unsupported input include config file: %+v", mode)
		return
	}
	return
}

// Takes file, loads, and then merges with existing config
func (opts *JSONInputs) mergeConfigInputs(fullPath string) (err error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return
	}

	var newCfg JSONInputs
	err = json.Unmarshal(data, &newCfg)
	if err != nil {
		err = fmt.Errorf("invalid JSON in include file %q: %w", fullPath, err)
		return
	}

	// Disallow cascading include
	if newCfg.Include != "" {
		err = fmt.Errorf("invalid included input config: recursive include directives are not permitted")
		return
	}

	// Override base config with include config

	if newCfg.JournalEnabled {
		opts.JournalEnabled = newCfg.JournalEnabled
	}

	for _, newPath := range newCfg.FilePaths {
		if slices.Contains(opts.FilePaths, newPath) {
			continue
		}
		opts.FilePaths = append(opts.FilePaths, newPath)
	}

	if opts.DropFilters == nil && newCfg.DropFilters == nil {
		return
	}
	if opts.DropFilters == nil && len(newCfg.DropFilters) > 0 {
		opts.DropFilters = make(map[string][]protocol.MessageFilter)
	}
	for key, newFilters := range newCfg.DropFilters {
		_, ok := opts.DropFilters[key]
		if !ok {
			// Key not present, just append to map
			opts.DropFilters[key] = append([]protocol.MessageFilter{}, newFilters...)
			continue
		}

		seen := make(map[string]struct{}, len(opts.DropFilters[key]))

		for _, existingFilters := range opts.DropFilters[key] {
			var existingFilterText []byte
			existingFilterText, err = json.Marshal(existingFilters)
			if err != nil {
				err = fmt.Errorf("failed to marshal filter for key %q: %w", key, err)
				return
			}
			seen[string(existingFilterText)] = struct{}{}
		}

		for _, newFilter := range newFilters {
			var newFilterText []byte
			newFilterText, err = json.Marshal(newFilter)
			if err != nil {
				err = fmt.Errorf("failed to marshal new filter for key %q: %w", key, err)
				return
			}

			_, exists := seen[string(newFilterText)]
			if exists {
				// Included filter has exact match for filter in main config
				continue
			}

			opts.DropFilters[key] = append(opts.DropFilters[key], newFilter)
			seen[string(newFilterText)] = struct{}{}
		}
	}

	return
}

// Sets defaults for any missing/invalid values
func (opts *JSONOptions) setDefaults() {
	// Crypto
	if opts.Crypto.TransportSuite == "" {
		// Default to only algo to use
		suiteInfo, valid := registry.GetSuiteInfo(1)
		if valid {
			opts.Crypto.TransportSuite = suiteInfo.Name
		}
	}
	if opts.Crypto.SignatureSuite == "" && opts.SigningKeyFile == "" {
		// Default to no signatures with no signer
		sigInfo, valid := registry.GetSignatureInfo(0)
		if valid {
			opts.Crypto.SignatureSuite = sigInfo.Name
		}
	} else if opts.Crypto.SignatureSuite == "" && opts.SigningKeyFile != "" {
		// Default to base signature with signing key
		sigInfo, valid := registry.GetSignatureInfo(1)
		if valid {
			opts.Crypto.SignatureSuite = sigInfo.Name
		}
	}

	// Scaling
	if time.Duration(opts.AutoScaling.PollInterval) < 1*time.Second {
		opts.AutoScaling.PollInterval = parsing.Duration(100 * time.Millisecond) // Enforced minimum
	}
	if time.Duration(opts.AutoScaling.PollInterval) > 1*time.Minute {
		// Longer times are not useful
		opts.AutoScaling.PollInterval = parsing.Duration(1 * time.Minute)
	}

	// Maximums
	logicalCPUCount := runtime.NumCPU()
	maxCPU := global.MaxValue(logicalCPUCount)
	minCPU := global.MinValue(logicalCPUCount)
	if opts.AutoScaling.MaxAssemblers == 0 {
		opts.AutoScaling.MaxAssemblers = maxCPU
	}
	if opts.AutoScaling.MaxOutputs == 0 {
		opts.AutoScaling.MaxOutputs = maxCPU
	}
	if opts.AutoScaling.MaxOutputQueueSize == 0 {
		opts.AutoScaling.MaxOutputQueueSize = global.DefaultMaxQueueSize
	}
	if opts.AutoScaling.MaxAssemblerQueueSize == 0 {
		opts.AutoScaling.MaxAssemblerQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if opts.AutoScaling.MinAssemblers == 0 {
		opts.AutoScaling.MinAssemblers = 1
	}
	if opts.AutoScaling.MinAssemblers > minCPU {
		opts.AutoScaling.MinAssemblers = minCPU
	}
	if opts.AutoScaling.MinOutputs == 0 {
		opts.AutoScaling.MinOutputs = 1
	}
	if opts.AutoScaling.MinOutputs > minCPU {
		opts.AutoScaling.MinOutputs = minCPU
	}
	if opts.AutoScaling.MinAssemblerQueueSize == 0 {
		opts.AutoScaling.MinAssemblerQueueSize = global.DefaultMinQueueSize
	}
	if opts.AutoScaling.MinOutputQueueSize == 0 {
		opts.AutoScaling.MinOutputQueueSize = global.DefaultMinQueueSize
	}

	if opts.State.BaseFile == "" {
		opts.State.BaseFile = global.DefaultStateFile
	}

	// Network
	if opts.Network.Port == 0 {
		opts.Network.Port = global.DefaultReceiverPort
	}

	// Metrics
	if opts.Metrics.MaxAge == 0 {
		opts.Metrics.MaxAge = parsing.Duration(1 * time.Hour)
	}
	if opts.Metrics.QueryServerPort == 0 {
		opts.Metrics.QueryServerPort = server.ListenPortSender
	}
	if opts.Metrics.Interval == 0 {
		opts.Metrics.Interval = parsing.Duration(15 * time.Second)
	}

	if opts.Throttling.MinFragmentThreshold == 0 {
		opts.Throttling.MinFragmentThreshold = DefaultOutputThrottlingThreshold
	}
	if opts.Throttling.PerFragmentDelay == 0 {
		opts.Throttling.PerFragmentDelay = parsing.Duration(DefaultOutputThrottlingTime)
	}
}

// Reads in signing private key from dedicated file
func loadSigningKey(keyPath string) (key []byte, err error) {
	var encodedSigningKey []byte
	encodedSigningKey, err = os.ReadFile(keyPath)
	if err != nil {
		err = fmt.Errorf("failed to read signing private key: %w", err)
		return
	}

	encodedSigningKey = bytes.Trim(encodedSigningKey, "\n")
	encodedSigningKey = bytes.TrimSpace(encodedSigningKey)

	key, err = base64.StdEncoding.DecodeString(string(encodedSigningKey))
	if err != nil {
		err = fmt.Errorf("failed to decode signing key: %w", err)
		return
	}
	return
}

// Reloads running sender daemon with new private signing key (all new outbound packets immediately start using it)
func (daemon *Daemon) ReloadSigningKeys() (diffCount int, err error) {
	oldCfg := daemon.opts
	err = daemon.LoadConfig(daemon.configPath)
	if err != nil {
		err = fmt.Errorf("failed to re-read daemon configuration file: %w", err)
		return
	}
	newCfg := daemon.opts
	daemon.opts = oldCfg

	if newCfg.SigningKeyFile == "" {
		// No-op
		return
	}
	// Write new key to daemon
	daemon.cfg.signingPrivateKey, err = loadSigningKey(newCfg.SigningKeyFile)
	if err != nil {
		return
	}

	// Update signing function
	if len(daemon.cfg.signingPrivateKey) > 0 {
		signatureSuiteID, validName := registry.SignatureNameToID(daemon.opts.Crypto.SignatureSuite)
		if !validName {
			err = fmt.Errorf("invalid signature suite name %s", daemon.opts.Crypto.SignatureSuite)
			return
		}
		info, validID := registry.GetSignatureInfo(signatureSuiteID)
		if !validID {
			err = fmt.Errorf("invalid signature suite ID: %d", signatureSuiteID)
			return
		}
		err = info.ValidateKey(daemon.cfg.signingPrivateKey)
		if err != nil {
			return
		}
		err = wrappers.SetupCreateSignature(daemon.cfg.signingPrivateKey)
		if err != nil {
			err = fmt.Errorf("failed to re-setup signing function: %w", err)
			return
		}
	}

	diffCount = 1 // Always one for sender daemon
	return
}
