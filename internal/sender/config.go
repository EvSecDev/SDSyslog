package sender

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/externalio/server"
	"sdsyslog/internal/global"
	"sdsyslog/pkg/crypto/registry"
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

	// Source settings
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
	if cfg.MaxAssemblers == 0 {
		cfg.MaxAssemblers = logicalCPUCount
	}
	if cfg.MaxOutputs == 0 {
		cfg.MaxOutputs = logicalCPUCount
	}
	if cfg.MaxOutputQueueSize == 0 {
		cfg.MaxOutputQueueSize = global.DefaultMaxQueueSize
	}
	if cfg.MaxAssemblerQueueSize == 0 {
		cfg.MaxAssemblerQueueSize = global.DefaultMaxQueueSize
	}

	// Minimums
	if cfg.MinAssemblers > logicalCPUCount {
		cfg.MinAssemblers = logicalCPUCount
	}
	if cfg.MinOutputs > logicalCPUCount {
		cfg.MinOutputs = logicalCPUCount
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
func (daemon *Daemon) ReloadSigningKeys() (newCount int, err error) {
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

	newCount = 1 // Always one for sender daemon
	return
}
