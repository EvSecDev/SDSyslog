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
	"sdsyslog/internal/externalio/server"
	"sdsyslog/internal/global"
	"sdsyslog/pkg/crypto/registry"
	"sdsyslog/pkg/protocol"
	"slices"
	"sort"
	"strings"
	"time"
)

// Loads JSON config from file
func LoadConfig(path string) (cfg JSONConfig, err error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("failed to read config file: %w", err)
		return
	}

	err = json.Unmarshal(configFile, &cfg)
	if err != nil {
		err = fmt.Errorf("invalid config syntax in '%s': %w", path, err)
		return
	}

	return
}

// Parses JSON config into daemon config
func (cfg JSONConfig) NewDaemonConf(originalConfigPath string) (config Config, err error) {
	config.path = originalConfigPath

	// Signatures
	if cfg.SigningKeyFile != "" {
		config.signingPrivateKey, err = loadSigningKey(cfg.SigningKeyFile)
		if err != nil {
			err = fmt.Errorf("failed to decode signing key: %w", err)
			return
		}
	}

	// Network settings
	config.DestinationIP = cfg.Network.Address
	config.DestinationPort = cfg.Network.Port
	config.OverrideMaxPayloadSize = cfg.Network.MaxPayloadSize

	// Input settings
	err = cfg.loadInputs() // Pull from file(s)
	if err != nil {
		return
	}
	config.Filters = cfg.Inputs.DropFilters
	config.StateFilePath = cfg.StateFile
	config.FileSourcePaths = cfg.Inputs.FilePaths
	config.JournalSourceEnabled = cfg.Inputs.JournalEnabled

	// Scaling settings
	config.AutoscaleCheckInterval, err = time.ParseDuration(cfg.AutoScaling.PollInterval)
	if err != nil {
		err = fmt.Errorf("failed to parse autoscale check interval time: %w", err)
		return
	}
	config.AutoscaleEnabled = cfg.AutoScaling.Enabled
	config.MinAssemblers = cfg.AutoScaling.MinAssemblers
	config.MaxAssemblers = cfg.AutoScaling.MaxAssemblers
	config.MinOutputs = cfg.AutoScaling.MinOutputs
	config.MaxOutputs = cfg.AutoScaling.MaxOutputs
	config.MinAssemblerQueueSize = cfg.AutoScaling.MinAssemblerQueueSize
	config.MaxAssemblerQueueSize = cfg.AutoScaling.MaxAssemblerQueueSize
	config.MinOutputQueueSize = cfg.AutoScaling.MinOutputQueueSize
	config.MaxOutputQueueSize = cfg.AutoScaling.MaxOutputQueueSize

	// Metric settings
	config.MetricQueryServerEnabled = cfg.Metrics.EnableQueryServer
	config.MetricQueryServerPort = cfg.Metrics.QueryServerPort
	config.MetricMaxAge, err = time.ParseDuration(cfg.Metrics.MaxAge)
	if err != nil {
		err = fmt.Errorf("failed to parse metric max age time: %w", err)
		return
	}
	config.MetricCollectionInterval, err = time.ParseDuration(cfg.Metrics.Interval)
	if err != nil {
		err = fmt.Errorf("failed to parse collection interval time: %w", err)
		return
	}

	return
}

// Loads all input configurations.
// Checks for input include directive and loads associated files
func (cfg *JSONConfig) loadInputs() (err error) {
	if cfg.Inputs.Include == "" {
		// No-op
		return
	}

	info, err := os.Stat(cfg.Inputs.Include)
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
		name := filepath.Base(cfg.Inputs.Include)
		if strings.HasPrefix(name, ".") {
			err = fmt.Errorf("input include file %q is hidden and will not be loaded", cfg.Inputs.Include)
			return
		}
		if filepath.Ext(name) != ".json" {
			err = fmt.Errorf("input include file %q does not have .json extension and will not be loaded", cfg.Inputs.Include)
			return
		}

		err = cfg.Inputs.mergeConfigInputs(cfg.Inputs.Include)
		if err != nil {
			err = fmt.Errorf("failed loading input include %q: %w", cfg.Inputs.Include, err)
			return
		}
	case mode.IsDir():
		var entries []os.DirEntry
		entries, err = os.ReadDir(cfg.Inputs.Include)
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

			fullPath := filepath.Join(cfg.Inputs.Include, name)
			err = cfg.Inputs.mergeConfigInputs(fullPath)
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
func (cfg *JSONInputs) mergeConfigInputs(fullPath string) (err error) {
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
		cfg.JournalEnabled = newCfg.JournalEnabled
	}

	for _, newPath := range newCfg.FilePaths {
		if slices.Contains(cfg.FilePaths, newPath) {
			continue
		}
		cfg.FilePaths = append(cfg.FilePaths, newPath)
	}

	if cfg.DropFilters == nil && newCfg.DropFilters == nil {
		return
	}
	if cfg.DropFilters == nil && len(newCfg.DropFilters) > 0 {
		cfg.DropFilters = make(map[string][]protocol.MessageFilter)
	}
	for key, newFilters := range newCfg.DropFilters {
		_, ok := cfg.DropFilters[key]
		if !ok {
			// Key not present, just append to map
			cfg.DropFilters[key] = append([]protocol.MessageFilter{}, newFilters...)
			continue
		}

		seen := make(map[string]struct{}, len(cfg.DropFilters[key]))

		for _, existingFilters := range cfg.DropFilters[key] {
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

			cfg.DropFilters[key] = append(cfg.DropFilters[key], newFilter)
			seen[string(newFilterText)] = struct{}{}
		}
	}

	return
}

// Sets defaults for any missing/invalid values
func (cfg *Config) setDefaults() {
	// Crypto
	cfg.transportCryptoSuiteID = 1
	if len(cfg.signingPrivateKey) > 0 {
		cfg.signatureSuiteID = 1 // Only supported algorithm
	} else {
		cfg.signatureSuiteID = 0 // No key to use
	}

	// Scaling
	if cfg.AutoscaleCheckInterval == 0 {
		cfg.AutoscaleCheckInterval = 5 * time.Second
	}

	// Maximums
	logicalCPUCount := runtime.NumCPU()
	maxCPU := global.MaxValue(logicalCPUCount)
	minCPU := global.MinValue(logicalCPUCount)
	if cfg.MaxAssemblers == 0 {
		cfg.MaxAssemblers = maxCPU
	}
	if cfg.MaxOutputs == 0 {
		cfg.MaxOutputs = maxCPU
	}
	if cfg.MaxOutputQueueSize == 0 {
		cfg.MaxOutputQueueSize = global.DefaultMaxQueueSize
	}
	if cfg.MaxAssemblerQueueSize == 0 {
		cfg.MaxAssemblerQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if cfg.MinAssemblers > minCPU {
		cfg.MinAssemblers = minCPU
	}
	if cfg.MinOutputs > minCPU {
		cfg.MinOutputs = minCPU
	}
	if cfg.MinAssemblerQueueSize == 0 {
		cfg.MinAssemblerQueueSize = global.DefaultMinQueueSize
	}
	if cfg.MinOutputQueueSize == 0 {
		cfg.MinOutputQueueSize = global.DefaultMinQueueSize
	}

	if cfg.StateFilePath == "" {
		cfg.StateFilePath = global.DefaultStateFile
	}

	// Network
	if cfg.DestinationPort == 0 {
		cfg.DestinationPort = global.DefaultReceiverPort
	}

	// Metrics
	if cfg.MetricMaxAge == 0 {
		cfg.MetricMaxAge = 1 * time.Hour
	}
	if cfg.MetricQueryServerPort == 0 {
		cfg.MetricQueryServerPort = server.ListenPortSender
	}
	if cfg.MetricCollectionInterval == 0 {
		cfg.MetricCollectionInterval = time.Duration(15 * time.Second)
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
	cfg, err := LoadConfig(daemon.cfg.path)
	if err != nil {
		err = fmt.Errorf("failed to re-read daemon configuration file: %w", err)
		return
	}
	if cfg.SigningKeyFile == "" {
		// No-op
		return
	}
	// Write new key to daemon
	daemon.cfg.signingPrivateKey, err = loadSigningKey(cfg.SigningKeyFile)
	if err != nil {
		return
	}

	// Update signing function
	if len(daemon.cfg.signingPrivateKey) > 0 {
		info, validID := registry.GetSignatureInfo(daemon.cfg.signatureSuiteID)
		if !validID {
			err = fmt.Errorf("invalid signature suite ID: %d", daemon.cfg.signatureSuiteID)
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
