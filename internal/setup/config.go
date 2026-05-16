package setup

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sdsyslog/internal/filtering"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/iomodules/beats"
	"sdsyslog/internal/iomodules/journald"
	"sdsyslog/internal/metrics/server"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/receiver"
	"sdsyslog/internal/sender"
	"sdsyslog/internal/sender/ingest"
	"sdsyslog/pkg/crypto/registry"
	"sdsyslog/pkg/protocol"
	"syscall"
	"time"
)

type InstallConfigStep struct {
	configPath     string
	dirCreated     bool
	configCreated  bool
	privKeyCreated bool
}

func (step *InstallConfigStep) Name() string {
	return "Configuration Files"
}

func (step *InstallConfigStep) NeedsApply(ctx *context) (alreadyInstalled bool, err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	switch ctx.mode {
	case global.SendMode:
		step.configPath = global.DefaultConfigSend
	case global.RecvMode:
		step.configPath = global.DefaultConfigRecv
	default:
		err = fmt.Errorf("unknown mode '%s'", ctx.mode)
		return
	}

	_, err = os.Stat(global.DefaultConfigDir)
	if err != nil && os.IsNotExist(err) {
		ctx.logger.Verbose("Configuration directory does not exist '%s'", global.DefaultConfigDir)
		alreadyInstalled = false
		err = nil
		return
	} else if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to check configuration directory: %w", err)
		return
	}

	// Never overwrite
	_, err = os.Stat(step.configPath)
	if err == nil {
		ctx.logger.Verbose("Configuration file '%s' present, refusing to overwrite", step.configPath)
		alreadyInstalled = true
		return
	}

	ctx.logger.Verbose("Configuration file '%s' not present", step.configPath)
	return
}

func (step *InstallConfigStep) Apply(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	ctx.logger.Verbose("Installing configuration file to '%s'", step.configPath)

	_, err = os.Stat(global.DefaultConfigDir)
	if err != nil && os.IsNotExist(err) {
		step.dirCreated = true
		err = os.Mkdir(global.DefaultConfigDir, 0755)
		if err != nil {
			err = fmt.Errorf("failed to create configuration directory: %w", err)
			return
		}
		ctx.logger.Verbose("Created configuration directory '%s'", global.DefaultConfigDir)
	} else if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to check configuration directory: %w", err)
		return
	}

	switch ctx.mode {
	case global.SendMode:
		err = CreateSendTemplateConfig(step.configPath)
		if err != nil {
			return
		}
		step.configCreated = true
	case global.RecvMode:
		info, validID := registry.GetSuiteInfo(ctx.suiteID)
		if !validID {
			err = fmt.Errorf("invalid suite ID %d", ctx.suiteID)
			return
		}

		var private, public []byte
		private, public, err = info.NewKey()
		if err != nil {
			err = fmt.Errorf("failed to generate keys: %w", err)
			return
		}

		_, err = os.Stat(encryptionPrivKeyPath)
		if err != nil {
			if !os.IsNotExist(err) {
				err = fmt.Errorf("failed checking private key file existence: %w", err)
				return
			}

			var privKeyFile *os.File
			privKeyFile, err = os.OpenFile(encryptionPrivKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				err = fmt.Errorf("failed to open private key file: %w", err)
				return
			}
			defer func() {
				_ = privKeyFile.Close()
			}()
			step.privKeyCreated = true

			_, err = privKeyFile.Write([]byte(base64.StdEncoding.EncodeToString(private)))
			if err != nil {
				err = fmt.Errorf("failed to write new private key: %w", err)
				return
			}
			ctx.logger.Info("Successfully wrote new private key file to '%s'", encryptionPrivKeyPath)
			ctx.logger.Info("  IMPORTANT: Public key (use this for all senders): %s", base64.StdEncoding.EncodeToString(public))
		} else {
			ctx.logger.Verbose("Private key file already exists at '%s'", encryptionPrivKeyPath)
		}

		err = CreateRecvTemplateConfig(step.configPath)
		if err != nil {
			return
		}
		step.configCreated = true
	default:
		err = fmt.Errorf("unknown mode '%s'", ctx.mode)
		return
	}

	ctx.logger.Success("Successfully wrote template configuration file to '%s'", step.configPath)
	return
}

func (step *InstallConfigStep) Rollback(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Remove private key if created (recv mode only)
	if step.privKeyCreated {
		err := os.Remove(encryptionPrivKeyPath)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove private key file '%s': %v", encryptionPrivKeyPath, err)
		} else {
			ctx.logger.Verbose("Removed private key file '%s'", encryptionPrivKeyPath)
		}
	}

	// Remove config file if we created it
	if step.configCreated {
		err := os.Remove(step.configPath)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove config file '%s': %v", step.configPath, err)
		} else {
			ctx.logger.Verbose("Removed configuration file '%s'", step.configPath)
		}
	}

	// Remove directory only if we created it AND it's empty
	if step.dirCreated {
		err := os.Remove(global.DefaultConfigDir)
		if err != nil && !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTEMPTY) {
			ctx.logger.Error("failed to remove config directory '%s': %v", global.DefaultConfigDir, err)
		} else {
			ctx.logger.Verbose("Removed configuration directory '%s'", global.DefaultConfigDir)
		}
	}
}

func (step *InstallConfigStep) PostApply(ctx *context) {
	// No-op
}

func (step *InstallConfigStep) Uninstall(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	if ctx.mode == global.RecvMode {
		err = os.Remove(encryptionPrivKeyPath)
		if err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("failed to remove private key file: %w", err)
			return
		}
		ctx.logger.Info("Successfully removed private key file '%s'", encryptionPrivKeyPath)
	}

	err = os.RemoveAll(global.DefaultConfigDir)
	if err != nil && !os.IsNotExist(err) {
		return
	} else {
		err = nil
	}

	ctx.logger.Success("Successfully removed configuration directory '%s'", global.DefaultConfigDir)
	return
}

func CreateSendTemplateConfig(filepath string) (err error) {
	if filepath == "" {
		err = fmt.Errorf("specify template file path via the --config/-c arguments")
		return
	}

	newConfFile, err := os.OpenFile(filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() {
		_ = newConfFile.Close()
	}()

	var newCfg sender.JSONOptions
	newCfg.AutoScaling.Enabled = true
	newCfg.AutoScaling.PollInterval = parsing.Duration(5 * time.Second)
	newCfg.AutoScaling.MinOutputs = 2
	newCfg.AutoScaling.MinAssemblers = 2
	newCfg.AutoScaling.MaxOutputs = 16
	newCfg.AutoScaling.MaxAssemblers = 16
	newCfg.AutoScaling.MinAssemblerQueueSize = global.DefaultMinQueueSize
	newCfg.AutoScaling.MaxAssemblerQueueSize = global.DefaultMaxQueueSize
	newCfg.AutoScaling.MinOutputQueueSize = global.DefaultMinQueueSize
	newCfg.AutoScaling.MaxOutputQueueSize = global.DefaultMaxQueueSize

	newCfg.State.BaseFile = global.DefaultStateFile

	newCfg.Inputs.Include = global.DefaultConfigDir + "/input-sender-extras.json"
	newCfg.Inputs.FilePaths = []string{"/var/log/nginx/kern.log"}
	newCfg.Inputs.JournalEnabled = true
	newCfg.Inputs.SendInternalLogs = true
	newCfg.Inputs.DropFilters = map[string][]protocol.MessageFilter{
		ingest.FileSource: {
			protocol.MessageFilter{
				FieldsKey: &filtering.Filter{
					Exact: "mycustomfield",
				},
				FieldsValue: &filtering.Filter{
					Contains: "value1",
				},
			},
		},
		ingest.JrnlSource: {
			protocol.MessageFilter{
				FieldsKey: &filtering.Filter{
					Exact: iomodules.CFfacility,
				},
				FieldsValue: &filtering.Filter{
					Exact: "ftp",
				},
				Data: &filtering.Filter{
					Or: []filtering.Filter{
						{
							Contains: "logout",
						},
						{
							Contains: "login",
						},
					},
				},
			},
			protocol.MessageFilter{
				FieldsKey: &filtering.Filter{
					Exact: iomodules.CFseverity,
				},
				FieldsValue: &filtering.Filter{
					Contains: iomodules.DefaultSeverity,
				},
				UseAnd: true,
			},
			protocol.MessageFilter{
				FieldsKey: &filtering.Filter{
					Exact: iomodules.CFseverity,
				},
				FieldsValue: &filtering.Filter{
					Contains: "debug",
				},
				UseAnd: true,
			},
			protocol.MessageFilter{
				FieldsKey: &filtering.Filter{
					Exact: iomodules.CFseverity,
				},
				FieldsValue: &filtering.Filter{
					Contains: "notice",
				},
				UseAnd: true,
			},
		},
	}

	newCfg.PublicKey = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx="

	newCfg.Crypto.SignatureSuite = registry.NoSigName
	newCfg.Crypto.TransportSuite = registry.DefaultCryptoName

	newCfg.Metrics.MaxAge = parsing.Duration(72 * time.Hour)
	newCfg.Metrics.Interval = parsing.Duration(5 * time.Second)
	newCfg.Metrics.QueryServerPort = server.ListenPortSender

	newCfg.Network.SourceAddress = "::1"
	newCfg.Network.SourcePort = 54321
	newCfg.Network.Address = "::1"
	newCfg.Network.Port = global.DefaultReceiverPort
	newCfg.Network.OverrideMaxPayloadSize = 1300

	confBytes, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		err = fmt.Errorf("error marshaling new config: %w", err)
		return
	}
	confBytes = append(confBytes, []byte("\n")...)

	_, err = newConfFile.Write(confBytes)
	if err != nil {
		err = fmt.Errorf("failed to write config to file: %w", err)
		return
	}
	return
}

func CreateRecvTemplateConfig(filepath string) (err error) {
	if filepath == "" {
		err = fmt.Errorf("specify template file path via the --config/-c arguments")
		return
	}

	newConfFile, err := os.OpenFile(filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() {
		_ = newConfFile.Close()
	}()

	var newCfg receiver.JSONOptions
	newCfg.AutoScaling.Enabled = true
	newCfg.AutoScaling.PollInterval = parsing.Duration(5 * time.Second)
	newCfg.AutoScaling.MinListeners = 2
	newCfg.AutoScaling.MinProcessors = 2
	newCfg.AutoScaling.MinDefrags = 2
	newCfg.AutoScaling.MaxListeners = 16
	newCfg.AutoScaling.MaxProcessors = 16
	newCfg.AutoScaling.MaxDefrags = 16

	// Queues
	newCfg.AutoScaling.MinProcQueueSize = global.DefaultMinQueueSize
	newCfg.AutoScaling.MaxProcQueueSize = global.DefaultMaxQueueSize
	newCfg.AutoScaling.MinOutQueueSize = global.DefaultMinQueueSize
	newCfg.AutoScaling.MaxOutQueueSize = global.DefaultMaxQueueSize

	newCfg.Outputs.FilePath = "/var/log/all.log"
	newCfg.Outputs.JournaldURL = journald.DefaultURL
	newCfg.Outputs.BeatsAddress = beats.DefaultAddress
	newCfg.Outputs.DBUSNotify = false

	newCfg.PrivateKeyFile = encryptionPrivKeyPath

	newCfg.Crypto.SignatureSuite = registry.NoSigName
	newCfg.Crypto.TransportSuite = registry.DefaultCryptoName

	newCfg.ReplayProtection.ProtectionWindow = parsing.Duration(receiver.DefaultReplayWindow)
	newCfg.ReplayProtection.PastValidityWindow = parsing.Duration(receiver.DefaultPastValidityWindow)
	newCfg.ReplayProtection.FutureValidityWindow = parsing.Duration(receiver.DefaultFutureValidityWindow)

	newCfg.State.IPCSocketDirectory = receiver.DefaultSocketDir

	newCfg.Metrics.MaxAge = parsing.Duration(72 * time.Hour)
	newCfg.Metrics.Interval = parsing.Duration(1 * time.Second)
	newCfg.Metrics.QueryServerPort = server.ListenPortReceiver

	newCfg.Network.Address = "::1"
	newCfg.Network.Port = global.DefaultReceiverPort

	confBytes, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		err = fmt.Errorf("error marshaling new config: %w", err)
		return
	}
	confBytes = append(confBytes, []byte("\n")...)

	_, err = newConfFile.Write(confBytes)
	if err != nil {
		err = fmt.Errorf("failed to write config to file: %w", err)
		return
	}
	return
}
