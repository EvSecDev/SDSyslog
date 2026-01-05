package sender

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
	config.DestinationIP = cfg.Network.Address
	config.DestinationPort = cfg.Network.Port
	config.OverrideMaxPayloadSize = cfg.Network.MaxPayloadSize

	// Source settings
	config.StateFilePath = cfg.StateFile
	config.FileSourcePaths = cfg.Inputs.FilePaths
	config.JournalSourceEnabled = cfg.Inputs.JournalEnabled

	// Scaling settings
	config.AutoscaleCheckInterval, err = time.ParseDuration(cfg.AutoScaling.PollInterval)
	if err != nil {
		err = fmt.Errorf("failed to parse autoscale check interval time: %v", err)
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
		err = fmt.Errorf("failed to parse metric max age time: %v", err)
		return
	}
	config.MetricCollectionInterval, err = time.ParseDuration(cfg.Metrics.Interval)
	if err != nil {
		err = fmt.Errorf("failed to parse collection interval time: %v", err)
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
	global.LogicalCPUCount = runtime.NumCPU()
	if cfg.MaxAssemblers == 0 {
		cfg.MaxAssemblers = global.LogicalCPUCount
	}
	if cfg.MaxOutputs == 0 {
		cfg.MaxOutputs = global.LogicalCPUCount
	}
	if cfg.MaxOutputQueueSize == 0 {
		cfg.MaxOutputQueueSize = global.DefaultMaxQueueSize
	}
	if cfg.MaxAssemblerQueueSize == 0 {
		cfg.MaxAssemblerQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if cfg.MinAssemblers > global.LogicalCPUCount {
		cfg.MinAssemblers = global.LogicalCPUCount
	}
	if cfg.MinOutputs > global.LogicalCPUCount {
		cfg.MinOutputs = global.LogicalCPUCount
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
		cfg.MetricQueryServerPort = global.HTTPListenPortSender
	}
	if cfg.MetricCollectionInterval == 0 {
		cfg.MetricCollectionInterval = time.Duration(15 * time.Second)
	}
}
