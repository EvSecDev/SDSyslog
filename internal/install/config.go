package install

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/global"
	"sdsyslog/internal/receiver"
	"sdsyslog/internal/sender"
	"strings"

	"golang.org/x/term"
)

func installConfig(mode string) (err error) {
	var configFilePath string
	switch mode {
	case "send":
		configFilePath = global.DefaultConfigSend
	case "receive":
		configFilePath = global.DefaultConfigRecv
	default:
		err = fmt.Errorf("unknown mode '%s'", mode)
		return
	}

	err = os.Mkdir(global.DefaultConfigDir, 0755)
	if err != nil {
		if strings.HasSuffix(err.Error(), "file exists") {
			err = nil
		} else {
			err = fmt.Errorf("failed to create configuration directory: %v", err)
			return
		}
	}

	// Don't overwrite existing
	_, err = os.Stat(configFilePath)
	if err == nil {
		// No terminal - no overwrite
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Printf("Existing configuration file present, not overwriting\n")
			return
		}

		// File exists, prompt user for confirmation to overwrite
		fmt.Printf("Configuration file already exists at '%s'. Are you SURE you want to overwrite it? (yes/no): ", configFilePath)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if strings.ToLower(input) != "yes" {
			fmt.Printf("Not overwriting configuration file\n")
			return
		}
	}

	switch mode {
	case "send":
		err = CreateSendTemplateConfig(configFilePath)
	case "receive":
		var private, public []byte
		private, public, err = ecdh.CreatePersistentKey()
		if err != nil {
			return
		}

		_, err = os.Stat(global.DefaultPrivKeyPath)
		if err != nil {
			if !os.IsNotExist(err) {
				err = fmt.Errorf("failed checking private key file existence: %v", err)
				return
			}

			var privKeyFile *os.File
			privKeyFile, err = os.OpenFile(global.DefaultPrivKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				err = fmt.Errorf("failed to open private key file: %v", err)
				return
			}

			_, err = privKeyFile.Write([]byte(base64.StdEncoding.EncodeToString(private)))
			if err != nil {
				err = fmt.Errorf("failed to write new private key: %v", err)
				return
			}
			fmt.Printf("Successfully wrote new private key file to '%s'\n", global.DefaultPrivKeyPath)
			fmt.Printf("  IMPORTANT: Public key (use this for all senders): %s\n", base64.StdEncoding.EncodeToString(public))
		}

		err = CreateRecvTemplateConfig(configFilePath)
	default:
		err = fmt.Errorf("unknown mode '%s'", mode)
		return
	}
	if err != nil {
		return
	}

	fmt.Printf("Successfully wrote template configuration file to '%s'\n", configFilePath)
	return
}

func uninstallConfig(mode string) (err error) {
	if mode == "receive" {
		err = os.Remove(global.DefaultPrivKeyPath)
		if err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("failed to remove private key file: %v", err)
			return
		} else {
			err = nil
		}
		fmt.Printf("Successfully removed private key file '%s'\n", global.DefaultPrivKeyPath)
	}

	err = os.RemoveAll(global.DefaultConfigDir)
	if err != nil && !os.IsNotExist(err) {
		return
	} else {
		err = nil
	}

	fmt.Printf("Successfully removed configuration directory '%s'\n", global.DefaultConfigDir)
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
	defer newConfFile.Close()

	var newCfg sender.JSONConfig
	newCfg.AutoScaling.Enabled = true
	newCfg.AutoScaling.PollInterval = "5s"
	newCfg.AutoScaling.MinOutputs = 2
	newCfg.AutoScaling.MinAssemblers = 2
	newCfg.AutoScaling.MaxOutputs = 16
	newCfg.AutoScaling.MaxAssemblers = 16
	newCfg.AutoScaling.MinAssemblerQueueSize = 1024
	newCfg.AutoScaling.MaxAssemblerQueueSize = 4096
	newCfg.AutoScaling.MinOutputQueueSize = 1024
	newCfg.AutoScaling.MaxOutputQueueSize = 4096

	newCfg.StateFile = global.DefaultStateFile

	newCfg.Inputs.FilePaths = []string{"/var/log/dmesg", "/var/log/nginx/access.log"}
	newCfg.Inputs.JournalEnabled = true

	newCfg.PublicKey = "xxxxxxxxxxxxxxxx=="

	newCfg.Metrics.MaxAge = "72h"
	newCfg.Metrics.Interval = "5s"
	newCfg.Metrics.QueryServerPort = global.HTTPListenPortSender

	newCfg.Network.Address = "[::1]"
	newCfg.Network.Port = global.DefaultReceiverPort
	newCfg.Network.MaxPayloadSize = 1300

	confBytes, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		err = fmt.Errorf("error marshaling new config: %v", err)
		return
	}
	confBytes = append(confBytes, []byte("\n")...)

	_, err = newConfFile.Write(confBytes)
	if err != nil {
		err = fmt.Errorf("failed to write config to file: %v", err)
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
	defer newConfFile.Close()

	var newCfg receiver.JSONConfig
	newCfg.AutoScaling.Enabled = true
	newCfg.AutoScaling.PollInterval = "5s"
	newCfg.AutoScaling.MinListeners = 2
	newCfg.AutoScaling.MinProcessors = 2
	newCfg.AutoScaling.MinDefrags = 16
	newCfg.AutoScaling.MaxListeners = 16
	newCfg.AutoScaling.MaxProcessors = 16
	newCfg.AutoScaling.MaxDefrags = 16

	newCfg.Outputs.FilePath = "/var/log/all.log"
	newCfg.Outputs.JournaldURL = global.DefaultJournaldURL
	newCfg.Outputs.BeatsAddress = global.DefaultBeatsAddr

	newCfg.PrivateKeyFile = global.DefaultPrivKeyPath

	newCfg.Metrics.MaxAge = "72h"
	newCfg.Metrics.Interval = "5s"
	newCfg.Metrics.QueryServerPort = global.HTTPListenPortReceiver

	newCfg.Network.Address = "[::1]"
	newCfg.Network.Port = global.DefaultReceiverPort

	confBytes, err := json.MarshalIndent(newCfg, "", "  ")
	if err != nil {
		err = fmt.Errorf("error marshaling new config: %v", err)
		return
	}
	confBytes = append(confBytes, []byte("\n")...)

	_, err = newConfFile.Write(confBytes)
	if err != nil {
		err = fmt.Errorf("failed to write config to file: %v", err)
		return
	}
	return
}
