package receiver

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/global"
	"time"
)

// Loads JSON config from file
func LoadConfig(path string) (cfg JSONConfig, err error) {
	configFile, err := os.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("failed to read config file: %v", err)
		return
	}

	err = json.Unmarshal(configFile, &cfg)
	if err != nil {
		err = fmt.Errorf("invalid config syntax in '%s': %v", path, err)
		return
	}

	return
}

// Parses JSON config into daemon config
func (cfg JSONConfig) NewDaemonConf() (config Config, err error) {
	// Network settings
	config.ListenIP = cfg.Network.Address
	config.ListenPort = cfg.Network.Port

	// Output settings
	config.JournaldURL = cfg.Outputs.JournaldURL
	config.OutputFilePath = cfg.Outputs.FilePath
	config.BeatsEndpoint = cfg.Outputs.BeatsAddress

	// Scaling settings
	config.AutoscaleEnabled = cfg.AutoScaling.Enabled
	config.AutoscaleCheckInterval, err = time.ParseDuration(cfg.AutoScaling.PollInterval)
	if err != nil {
		err = fmt.Errorf("failed to parse autoscale check interval time: %v", err)
		return
	}
	config.MinListeners = cfg.AutoScaling.MinListeners
	config.MinProcessors = cfg.AutoScaling.MinProcessors
	config.MinDefrags = cfg.AutoScaling.MinDefrags
	config.MaxListeners = cfg.AutoScaling.MaxListeners
	config.MaxProcessors = cfg.AutoScaling.MaxProcessors
	config.MaxDefrags = cfg.AutoScaling.MaxDefrags

	// Metric settings
	config.MetricQueryServerEnabled = cfg.Metrics.EnableQueryServer
	config.MetricQueryServerPort = cfg.Metrics.QueryServerPort
	config.MetricMaxAge, err = time.ParseDuration(cfg.Metrics.MaxAge)
	if err != nil {
		err = fmt.Errorf("failed to parse metric max age time: %v", err)
		return
	}
	config.MetricCollectionInterval, err = time.ParseDuration(cfg.Metrics.Interval)
	if err != nil {
		err = fmt.Errorf("failed to parse metric collection interval time: %v", err)
		return
	}
	return
}

// Sets defaults for any missing/invalid values
func (cfg *Config) setDefaults() {
	// Scaling
	if cfg.AutoscaleCheckInterval == 0 {
		cfg.AutoscaleCheckInterval = 5 * time.Second
	}
	// Maximums
	logicalCPUCount := runtime.NumCPU()
	if cfg.MaxListeners == 0 {
		cfg.MaxListeners = logicalCPUCount
	}
	if cfg.MaxProcessors == 0 {
		cfg.MaxProcessors = logicalCPUCount
	}
	if cfg.MaxDefrags == 0 {
		cfg.MaxDefrags = logicalCPUCount
	}
	if cfg.MaxProcessorQueueSize == 0 {
		cfg.MaxProcessorQueueSize = global.DefaultMaxQueueSize
	}
	if cfg.MaxOutputQueueSize == 0 {
		cfg.MaxOutputQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if cfg.MinDefrags > logicalCPUCount {
		cfg.MinDefrags = logicalCPUCount
	}
	if cfg.MinListeners > logicalCPUCount {
		cfg.MinListeners = logicalCPUCount
	}
	if cfg.MinProcessors > logicalCPUCount {
		cfg.MinProcessors = logicalCPUCount
	}
	if cfg.MinProcessorQueueSize == 0 {
		cfg.MinProcessorQueueSize = global.DefaultMinQueueSize
	}
	if cfg.MinOutputQueueSize == 0 {
		cfg.MinOutputQueueSize = global.DefaultMinQueueSize
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
		cfg.MetricQueryServerPort = global.HTTPListenPortReceiver
	}
	if cfg.MetricCollectionInterval == 0 {
		cfg.MetricCollectionInterval = time.Duration(15 * time.Second)
	}
}
