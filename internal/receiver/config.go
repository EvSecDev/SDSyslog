package receiver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/metrics/server"
	"sdsyslog/internal/parsing"
	"sdsyslog/pkg/crypto/registry"
	"slices"
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
func (daemon *Daemon) LoadKey() (key []byte, err error) {
	privateKey, err := os.ReadFile(daemon.opts.PrivateKeyFile)
	if err != nil {
		err = fmt.Errorf("failed reading private key file: %w", err)
		return
	}

	key, err = base64.StdEncoding.DecodeString(string(privateKey))
	if err != nil {
		err = fmt.Errorf("failed decoding private key file: %w", err)
		return
	}
	return
}

// Sets defaults for any missing/invalid values
func (opts *JSONOptions) setDefaults() {
	if opts.Crypto.TransportSuite == "" {
		// Default to only algo to use
		suiteInfo, valid := registry.GetSuiteInfo(1)
		if valid {
			opts.Crypto.TransportSuite = suiteInfo.Name
		}
	}
	if opts.Crypto.SignatureSuite == "" && opts.PinnedSigningKeysPath == "" {
		// Default to no signatures with no signer public keys
		sigInfo, valid := registry.GetSignatureInfo(0)
		if valid {
			opts.Crypto.SignatureSuite = sigInfo.Name
		}
	} else if opts.Crypto.SignatureSuite == "" && opts.PinnedSigningKeysPath != "" {
		// Default to base signature with signing key map
		sigInfo, valid := registry.GetSignatureInfo(1)
		if valid {
			opts.Crypto.SignatureSuite = sigInfo.Name
		}
	}

	// Scaling
	if opts.AutoScaling.PollInterval == 0 {
		opts.AutoScaling.PollInterval = parsing.Duration(5 * time.Second)
	}
	// Maximums
	logicalCPUCount := runtime.NumCPU()
	maxCPU := global.MaxValue(logicalCPUCount)
	minCPU := global.MinValue(logicalCPUCount)
	if opts.AutoScaling.MaxListeners == 0 {
		opts.AutoScaling.MaxListeners = maxCPU
	}
	if opts.AutoScaling.MaxProcessors == 0 {
		opts.AutoScaling.MaxProcessors = maxCPU
	}
	if opts.AutoScaling.MaxDefrags == 0 {
		opts.AutoScaling.MaxDefrags = maxCPU
	}
	if opts.AutoScaling.MaxProcQueueSize == 0 {
		opts.AutoScaling.MaxProcQueueSize = global.DefaultMaxQueueSize
	}
	if opts.AutoScaling.MaxOutQueueSize == 0 {
		opts.AutoScaling.MaxOutQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if opts.AutoScaling.MinDefrags == 0 {
		opts.AutoScaling.MinDefrags = 1
	}
	if opts.AutoScaling.MinDefrags > minCPU {
		opts.AutoScaling.MinDefrags = minCPU
	}
	if opts.AutoScaling.MinListeners == 0 {
		opts.AutoScaling.MinListeners = 1
	}
	if opts.AutoScaling.MinListeners > minCPU {
		opts.AutoScaling.MinListeners = minCPU
	}
	if opts.AutoScaling.MinProcessors == 0 {
		opts.AutoScaling.MinProcessors = 1
	}
	if opts.AutoScaling.MinProcessors > minCPU {
		opts.AutoScaling.MinProcessors = minCPU
	}
	if opts.AutoScaling.MinProcQueueSize == 0 {
		opts.AutoScaling.MinProcQueueSize = global.DefaultMinQueueSize
	}
	if opts.AutoScaling.MinOutQueueSize == 0 {
		opts.AutoScaling.MinOutQueueSize = global.DefaultMinQueueSize
	}

	// Message validity
	if opts.ReplayProtection.ProtectionWindow == 0 {
		opts.ReplayProtection.ProtectionWindow = parsing.Duration(DefaultReplayWindow)
	}
	if opts.ReplayProtection.PastValidityWindow == 0 {
		opts.ReplayProtection.PastValidityWindow = parsing.Duration(DefaultPastValidityWindow)
	}
	if opts.ReplayProtection.FutureValidityWindow == 0 {
		opts.ReplayProtection.FutureValidityWindow = parsing.Duration(DefaultFutureValidityWindow)
	}

	// Paths
	if opts.State.IPCSocketDirectory == "" {
		opts.State.IPCSocketDirectory = DefaultSocketDir
	}

	// Network
	if opts.Network.Address == "" {
		opts.Network.Address = "::"
	}
	if opts.Network.Port == 0 {
		opts.Network.Port = global.DefaultReceiverPort
	}

	// Metrics
	if opts.Metrics.MaxAge == 0 {
		opts.Metrics.MaxAge = parsing.Duration(1 * time.Hour)
	}
	if opts.Metrics.QueryServerPort == 0 {
		opts.Metrics.QueryServerPort = server.ListenPortReceiver
	}
	if opts.Metrics.Interval == 0 {
		opts.Metrics.Interval = parsing.Duration(15 * time.Second)
	}

	// Scaler
	if time.Duration(opts.AutoScaling.PollInterval) < 1*time.Second {
		// Protect routing algorithm from multi-step scaling within packet deadline
		opts.AutoScaling.PollInterval = parsing.Duration(2 * global.DefaultMaxPacketDeadline)
	}
	if time.Duration(opts.AutoScaling.PollInterval) > 1*time.Minute {
		// Longer times are not useful
		opts.AutoScaling.PollInterval = parsing.Duration(1 * time.Minute)
	}
}

// Loads newest config from disk and pulls newest pinned keys map.
func (daemon *Daemon) ReloadSigningKeys() (diffCount int, err error) {
	oldCfg := daemon.opts
	err = daemon.LoadConfig(daemon.configPath)
	if err != nil {
		err = fmt.Errorf("failed to re-read daemon configuration file: %w", err)
		return
	}
	newCfg := daemon.opts
	daemon.opts = oldCfg
	if newCfg.PinnedSigningKeysPath == "" {
		// No-op
		return
	}
	pinnedKeysFile, err := os.ReadFile(newCfg.PinnedSigningKeysPath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed reading pinned keys file: %w", err)
		return
	} else if err != nil && os.IsNotExist(err) {
		// No-op
		return
	}
	var newPinnedKeys map[string][]byte
	err = json.Unmarshal(pinnedKeysFile, &newPinnedKeys)
	if err != nil {
		err = fmt.Errorf("failed to parse pinned keys JSON: %w", err)
		return
	}
	if len(newPinnedKeys) == 0 {
		// No more pinned keys - zero out in-use map
		newPinnedKeys = make(map[string][]byte)
	}
	// Update in-use map
	wrappers.NewPinnedSenders(newPinnedKeys)

	// Find differences to count modifications (adds or deletes)
	oldPinnedKeys := daemon.cfg.PinnedSigningKeys
	for newKey, newVal := range newPinnedKeys {
		oldVal, present := oldPinnedKeys[newKey]
		if !present {
			diffCount++
			continue
		}
		if !slices.Equal(oldVal, newVal) {
			diffCount++
			continue
		}
	}
	for oldKey := range oldPinnedKeys {
		_, present := newPinnedKeys[oldKey]
		if !present {
			diffCount++
			continue
		}
	}

	daemon.cfg.PinnedSigningKeys = newPinnedKeys
	return
}
