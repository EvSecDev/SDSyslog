package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sdsyslog/internal/fsops"
	"sdsyslog/internal/global"
	"strings"
)

type InstallSystemdStep struct {
	templateUnitFile string // embed fs path
	serviceUnitFile  string
	installedNew     bool
	backupUnitFile   string
	backupCreated    bool
}

func (step *InstallSystemdStep) applyTemplateMacros(unitFile []byte) (file []byte) {
	// Inject variables into file
	newUnitFile := strings.Replace(string(unitFile), "$executableFilePath", global.DefaultBinaryPath, 1)
	newUnitFile = strings.Replace(newUnitFile, "$receiveConfigFilePath", global.DefaultConfigRecv, 1)
	newUnitFile = strings.Replace(newUnitFile, "$sendConfigFilePath", global.DefaultConfigSend, 1)
	file = []byte(newUnitFile)
	return
}

func (step *InstallSystemdStep) Name() string {
	return "Systemd Service"
}

func (step *InstallSystemdStep) NeedsApply(ctx *context) (alreadyDone bool, err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	switch ctx.mode {
	case global.SendMode:
		step.serviceUnitFile = senderUnitPath
		step.templateUnitFile = "static-files/" + global.ProgBaseName + "-sender.service"
	case global.RecvMode:
		step.serviceUnitFile = receiverUnitPath
		step.templateUnitFile = "static-files/" + global.ProgBaseName + ".service"
	default:
		err = fmt.Errorf("unknown mode '%s'", ctx.mode)
		return
	}

	unitFile, err := installationFiles.ReadFile(step.templateUnitFile)
	if err != nil {
		err = fmt.Errorf("unable to retrieve configuration file from embedded filesystem: %w", err)
		return
	}
	unitFile = step.applyTemplateMacros(unitFile)
	templateReader := bytes.NewReader(unitFile)

	isEqual, err := fsops.FileEqualsReader(step.serviceUnitFile, templateReader)
	if err != nil {
		return
	}
	if isEqual {
		ctx.logger.Verbose("Systemd service unit file '%s' is up-to-date", step.serviceUnitFile)
		alreadyDone = true
		return
	}

	ctx.logger.Verbose("Systemd service unit file '%s' differs from embedded file", step.serviceUnitFile)
	return
}

func (step *InstallSystemdStep) Apply(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	unitName := filepath.Base(step.serviceUnitFile)

	ctx.logger.Verbose("Installing systemd service %s to file '%s'", unitName, step.serviceUnitFile)

	unitFile, err := installationFiles.ReadFile(step.templateUnitFile)
	if err != nil {
		err = fmt.Errorf("unable to retrieve configuration file from embedded filesystem: %w", err)
		return
	}
	unitFile = step.applyTemplateMacros(unitFile)

	step.backupUnitFile, step.backupCreated, err = fsops.MakeFileBackup(step.serviceUnitFile)
	if err != nil {
		err = fmt.Errorf("failed to backup existing unit file: %w", err)
		return
	}

	err = os.WriteFile(step.serviceUnitFile, unitFile, 0644)
	if err != nil {
		err = fmt.Errorf("failed to write new unit file: %w", err)
		return
	}
	step.installedNew = true

	// Reload for new unit file
	command := exec.Command("systemctl", "daemon-reload")
	output, err := command.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to reload systemd units: %w: %s", err, string(output))
		return
	}

	ctx.logger.Success("Successfully installed Systemd service")
	ctx.logger.Info("  IMPORTANT: modify the configuration to your needs and start the service with 'systemctl start %s'", unitName)
	return
}

func (step *InstallSystemdStep) Rollback(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Remove installed unit file if we installed one
	if step.installedNew {
		err := os.Remove(step.serviceUnitFile)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove service unit file: %v", err)
		}
	}

	// Previous a file at target path, move back into place
	if step.backupCreated {
		ctx.logger.Verbose("Restoring previous unit file from '%s' to '%s'", step.backupUnitFile, step.serviceUnitFile)

		err := os.Rename(step.backupUnitFile, step.serviceUnitFile)
		if err != nil {
			ctx.logger.Error("failed to restore backup: %v", err)
		} else {
			dir, err := os.Open(filepath.Dir(step.serviceUnitFile))
			if err != nil {
				ctx.logger.Error("failed to open unit file directory: %v", err)
			} else {
				err = dir.Sync()
				if err != nil {
					ctx.logger.Error("failed to sync unit file directory: %v", err)
				}
				_ = dir.Close()
			}
		}
	}
}

// Enabling service if install succeeded (don't want service enabled if install fails somewhere)
func (step *InstallSystemdStep) PostApply(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	var unitFilePath string
	switch ctx.mode {
	case global.SendMode:
		unitFilePath = senderUnitPath
	case global.RecvMode:
		unitFilePath = receiverUnitPath
	default:
		ctx.logger.Error("unknown mode '%s'", ctx.mode)
		return
	}
	unitName := filepath.Base(unitFilePath)

	if step.backupCreated {
		err := os.Remove(step.backupUnitFile)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove previous unit file backup: %v", err)
			return
		}
	}

	// Check if enabled
	command := exec.Command("systemctl", "is-enabled", unitName)
	output, err := command.CombinedOutput()
	enableStatus := strings.Trim(string(output), "\n")
	enableStatus = strings.TrimSpace(enableStatus)
	if err != nil {
		if enableStatus != "disabled" {
			ctx.logger.Error("failed to check systemd service enablement status: %v: %s", err, enableStatus)
			return
		}
		// Disabled status is exit code 1
	}

	if strings.ToLower(enableStatus) != "enabled" {
		command := exec.Command("systemctl", "enable", unitName)
		output, err = command.CombinedOutput()
		if err != nil {
			ctx.logger.Error("failed to enable systemd service: %v: %s", err, string(output))
			return
		}
	}
}

func (step *InstallSystemdStep) Uninstall(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	var unitFilePath string
	switch ctx.mode {
	case global.SendMode:
		unitFilePath = senderUnitPath
	case global.RecvMode:
		unitFilePath = receiverUnitPath
	default:
		ctx.logger.Error("unknown mode '%s'", ctx.mode)
		return
	}
	unitName := filepath.Base(unitFilePath)

	// Check if enabled
	command := exec.Command("systemctl", "is-enabled", unitName)
	output, err := command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "not-found") && !strings.Contains(string(output), "disabled") {
			if !strings.Contains(string(output), "disabled") && !strings.Contains(string(output), "enabled") {
				err = fmt.Errorf("failed to check systemd service enablement status: %w: %s", err, string(output))
				return
			}
		}
		// Disabled/not-found status is exit code != 0
	}
	enableStatus := strings.Trim(string(output), "\n")

	if strings.ToLower(enableStatus) == "enabled" {
		command := exec.Command("systemctl", "disable", unitName)
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("failed to disable systemd service: %w: %s", err, string(output))
			return
		}
		ctx.logger.Verbose("Systemd service unit %s disabled", unitName)
	}

	command = exec.Command("systemctl", "show", unitName, "--property=ActiveState")
	output, err = command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "could not be found") {
			err = fmt.Errorf("failed to check systemd service status: %w: %s", err, string(output))
			return
		}
	}
	serviceStatus := strings.Trim(string(output), "\n")
	serviceStatus = strings.ToLower(serviceStatus)

	if strings.Contains(serviceStatus, "activestate=active") {
		command = exec.Command("systemctl", "stop", unitName)
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("failed to stop systemd service: %w: %s", err, string(output))
			return
		}
		ctx.logger.Verbose("Systemd service unit %s stopped", unitName)
	}

	err = os.Remove(unitFilePath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed removing unit file: %w", err)
		return
	}
	err = nil
	removed := true

	if removed {
		// Reload for removed unit file
		command = exec.Command("systemctl", "daemon-reload")
		output, err = command.CombinedOutput()
		if err != nil {
			err = fmt.Errorf("failed to reload systemd units: %w: %s", err, string(output))
			return
		}
	}

	ctx.logger.Success("Successfully uninstalled systemd service")
	return
}
