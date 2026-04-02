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
	"slices"
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

	// Network settings
	config.ListenIP = cfg.Network.Address
	config.ListenPort = cfg.Network.Port

	// Load pinned keys
	if cfg.PinnedSigningKeysPath != "" {
		var data []byte
		data, err = os.ReadFile(cfg.PinnedSigningKeysPath)
		if err != nil {
			err = fmt.Errorf("failed to read pinned signing keys file: %w", err)
			return
		}

		var raw map[string]string
		err = json.Unmarshal(data, &raw)
		if err != nil {
			err = fmt.Errorf("invalid pinned signing keys file format: %w", err)
			return
		}

		config.PinnedSigningKeysFile = cfg.PinnedSigningKeysPath

		config.PinnedSigningKeys = make(map[string][]byte, len(raw))
		for host, b64 := range raw {
			var key []byte
			key, err = base64.StdEncoding.DecodeString(b64)
			if err != nil {
				err = fmt.Errorf("invalid key for sender hostname %s (must be base64): %w", host, err)
				return
			}
			config.PinnedSigningKeys[host] = key
		}
	} else {
		config.PinnedSigningKeys = make(map[string][]byte)
	}

	// Output settings
	config.JournaldURL = cfg.Outputs.JournaldURL
	config.OutputFilePath = cfg.Outputs.FilePath
	config.BeatsEndpoint = cfg.Outputs.BeatsAddress

	// Scaling settings
	config.AutoscaleEnabled = cfg.AutoScaling.Enabled
	config.AutoscaleCheckInterval, err = time.ParseDuration(cfg.AutoScaling.PollInterval)
	if err != nil {
		err = fmt.Errorf("failed to parse autoscale check interval time: %w", err)
		return
	}
	config.MinListeners = cfg.AutoScaling.MinListeners
	config.MinProcessors = cfg.AutoScaling.MinProcessors
	config.MinDefrags = cfg.AutoScaling.MinDefrags
	config.MaxListeners = cfg.AutoScaling.MaxListeners
	config.MaxProcessors = cfg.AutoScaling.MaxProcessors
	config.MaxDefrags = cfg.AutoScaling.MaxDefrags

	// Queues
	config.MinProcessorQueueSize = cfg.AutoScaling.MinProcQueueSize
	config.MaxProcessorQueueSize = cfg.AutoScaling.MaxProcQueueSize
	config.MinOutputQueueSize = cfg.AutoScaling.MinOutQueueSize
	config.MaxOutputQueueSize = cfg.AutoScaling.MaxOutQueueSize

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
		err = fmt.Errorf("failed to parse metric collection interval time: %w", err)
		return
	}
	return
}

// Sets defaults for any missing/invalid values
func (cfg *Config) setDefaults() {
	// Default - only algo to use
	cfg.transportCryptoSuiteID = 1

	// Scaling
	if cfg.AutoscaleCheckInterval == 0 {
		cfg.AutoscaleCheckInterval = 5 * time.Second
	}
	// Maximums
	logicalCPUCount := runtime.NumCPU()
	maxCPU := global.MaxValue(logicalCPUCount)
	minCPU := global.MinValue(logicalCPUCount)
	if cfg.MaxListeners == 0 {
		cfg.MaxListeners = maxCPU
	}
	if cfg.MaxProcessors == 0 {
		cfg.MaxProcessors = maxCPU
	}
	if cfg.MaxDefrags == 0 {
		cfg.MaxDefrags = maxCPU
	}
	if cfg.MaxProcessorQueueSize == 0 {
		cfg.MaxProcessorQueueSize = global.DefaultMaxQueueSize
	}
	if cfg.MaxOutputQueueSize == 0 {
		cfg.MaxOutputQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if cfg.MinDefrags == 0 {
		cfg.MinDefrags = 1
	}
	if cfg.MinDefrags > minCPU {
		cfg.MinDefrags = minCPU
	}
	if cfg.MinListeners == 0 {
		cfg.MinListeners = 1
	}
	if cfg.MinListeners > minCPU {
		cfg.MinListeners = minCPU
	}
	if cfg.MinProcessors == 0 {
		cfg.MinProcessors = 1
	}
	if cfg.MinProcessors > minCPU {
		cfg.MinProcessors = minCPU
	}
	if cfg.MinProcessorQueueSize == 0 {
		cfg.MinProcessorQueueSize = global.DefaultMinQueueSize
	}
	if cfg.MinOutputQueueSize == 0 {
		cfg.MinOutputQueueSize = global.DefaultMinQueueSize
	}

	// Message validity
	if cfg.ReplayProtectionWindow == 0 {
		cfg.ReplayProtectionWindow = DefaultReplayWindow
	}
	if cfg.PastValidityWindow == 0 {
		cfg.PastValidityWindow = DefaultPastValidityWindow
	}
	if cfg.FutureValidityWindow == 0 {
		cfg.FutureValidityWindow = DefaultFutureValidityWindow
	}

	// Paths
	if cfg.SocketDirectoryPath == "" {
		cfg.SocketDirectoryPath = DefaultSocketDir
	}

	// Network
	if cfg.ListenIP == "" {
		cfg.ListenIP = "::"
	}
	if cfg.ListenPort == 0 {
		cfg.ListenPort = global.DefaultReceiverPort
	}

	// Metrics
	if cfg.MetricMaxAge == 0 {
		cfg.MetricMaxAge = 1 * time.Hour
	}
	if cfg.MetricQueryServerPort == 0 {
		cfg.MetricQueryServerPort = server.ListenPortReceiver
	}
	if cfg.MetricCollectionInterval == 0 {
		cfg.MetricCollectionInterval = time.Duration(15 * time.Second)
	}

	// Scaler
	if cfg.AutoscaleCheckInterval < 1*time.Second {
		// Protect routing algorithm from multi-step scaling within packet deadline
		cfg.AutoscaleCheckInterval = 2 * global.DefaultMaxPacketDeadline
	}
	if cfg.AutoscaleCheckInterval > 1*time.Minute {
		// Longer times are not useful
		cfg.AutoscaleCheckInterval = 1 * time.Minute
	}
}

// Loads newest config from disk and pulls newest pinned keys map.
func (daemon *Daemon) ReloadSigningKeys() (diffCount int, err error) {
	cfg, err := LoadConfig(daemon.cfg.path)
	if err != nil {
		err = fmt.Errorf("failed to re-read daemon configuration file: %w", err)
		return
	}
	if cfg.PinnedSigningKeysPath == "" {
		// No-op
		return
	}
	pinnedKeysFile, err := os.ReadFile(cfg.PinnedSigningKeysPath)
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
