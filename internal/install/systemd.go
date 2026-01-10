package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sdsyslog/internal/global"
	"strings"
)

func installService(mode string) (err error) {
	var unitFilePath string
	var templateFile string
	switch mode {
	case "send":
		unitFilePath = "/etc/systemd/system/sdsyslog-send.service"
		templateFile = "static-files/sdsyslog-sender.service"
	case "receive":
		unitFilePath = "/etc/systemd/system/sdsyslog.service"
		templateFile = "static-files/sdsyslog.service"
	default:
		err = fmt.Errorf("unknown mode '%s'", mode)
		return
	}
	unitName := filepath.Base(unitFilePath)

	unitFile, err := installationFiles.ReadFile(templateFile)
	if err != nil {
		err = fmt.Errorf("Unable to retrieve configuration file from embedded filesystem: %v", err)
		return
	}

	// Inject variables into file
	newUnitFile := strings.Replace(string(unitFile), "$executableFilePath", global.DefaultBinaryPath, 1)
	newUnitFile = strings.Replace(newUnitFile, "$receiveConfigFilePath", global.DefaultConfigRecv, 1)
	newUnitFile = strings.Replace(newUnitFile, "$sendConfigFilePath", global.DefaultConfigSend, 1)
	unitFile = []byte(newUnitFile)

	err = os.WriteFile(unitFilePath, unitFile, 0644)
	if err != nil {
		return
	}

	// Reload for new unit file
	command := exec.Command("systemctl", "daemon-reload")
	output, err := command.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Failed to reload systemd units: %v: %s", err, string(output))
		return
	}

	// Check if enabled
	command = exec.Command("systemctl", "is-enabled", unitName)
	output, err = command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "disabled") {
			err = fmt.Errorf("Failed to check systemd service enablement status: %v: %s", err, string(output))
			return
		}
		// Disabled status is exit code 1
		err = nil
	}
	enableStatus := strings.Trim(string(output), "\n")

	if strings.ToLower(enableStatus) != "enabled" {
		command := exec.Command("systemctl", "enable", unitName)
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("Failed to enable systemd service: %v: %s", err, string(output))
			return
		}
	}

	fmt.Printf("Successfully installed Systemd service\n")
	fmt.Printf("  IMPORTANT: modify the configuration to your needs and start the service with 'systemctl start %s'\n", unitName)
	return
}

func uninstallService(mode string) (err error) {
	var unitFilePath string
	switch mode {
	case "send":
		unitFilePath = "/etc/systemd/system/sdsyslog-send.service"
	case "receive":
		unitFilePath = "/etc/systemd/system/sdsyslog.service"
	default:
		err = fmt.Errorf("unknown mode '%s'", mode)
		return
	}
	unitName := filepath.Base(unitFilePath)

	// Check if enabled
	command := exec.Command("systemctl", "is-enabled", unitName)
	output, err := command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "not-found") && !strings.Contains(string(output), "disabled") {
			if !strings.Contains(string(output), "disabled") && !strings.Contains(string(output), "enabled") {
				err = fmt.Errorf("Failed to check systemd service enablement status: %v: %s", err, string(output))
				return
			}
		}
		// Disabled/not-found status is exit code != 0
		err = nil
	}
	enableStatus := strings.Trim(string(output), "\n")

	if strings.ToLower(enableStatus) == "enabled" {
		command := exec.Command("systemctl", "disable", unitName)
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("Failed to disable systemd service: %v: %s", err, string(output))
			return
		}
	}

	command = exec.Command("systemctl", "show", unitName, "--property=ActiveState")
	output, err = command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "could not be found") {
			err = fmt.Errorf("Failed to check systemd service status: %v: %s", err, string(output))
			return
		}
	}
	serviceStatus := strings.Trim(string(output), "\n")

	if strings.Contains(serviceStatus, "running") {
		command = exec.Command("systemctl", "stop", unitName)
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("Failed to stop systemd service: %v: %s", err, string(output))
			return
		}
	}

	err = os.Remove(unitFilePath)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	if os.IsNotExist(err) {
		err = nil // reset remove error

		// Reload for removed unit file
		command = exec.Command("systemctl", "daemon-reload")
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("Failed to reload systemd units: %v: %s", err, string(output))
			return
		}
	}

	fmt.Printf("Successfully uninstalled systemd service\n")
	return
}
