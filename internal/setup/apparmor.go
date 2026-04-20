package setup

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/fsops"
	"sdsyslog/internal/global"
	"strings"
)

type InstallAppArmorStep struct {
	installedNew      bool
	backupProfileFile string
	backupCreated     bool
	profileApplied    bool
}

func (step *InstallAppArmorStep) applyTemplateMacros(appArmorProfile []byte) (file []byte) {
	// Inject variables into config
	replacer := strings.NewReplacer(
		"=$executableFilePath", "="+global.DefaultBinaryPath,
		"=$configurationDirPath", "="+global.DefaultConfigDir,
		"=$privateKeyFilePath", "="+encryptionPrivKeyPath,
		"=$progStateDirPath", "="+global.DefaultStateDir,
		"=$drainingSocketsMapPinPath", "="+ebpf.KernelDrainMapPath,
		"=$drainingSocketsFuncPinPath", "="+ebpf.KernelSocketRouteFunc,
		"$includeExtraLocalPath", appArmorExtrasPath,
	)
	file = []byte(replacer.Replace(string(appArmorProfile)))
	return
}

func (step *InstallAppArmorStep) Name() string {
	return "AppArmor Profile"
}

func (step *InstallAppArmorStep) NeedsApply(ctx *context) (alreadyDone bool, err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Check if apparmor /sys path exists
	_, err = os.Stat(sysAAProfilePath)
	if err != nil && os.IsNotExist(err) {
		ctx.logger.Verbose("AppArmor not supported by this system")
		alreadyDone = true
		return
	} else if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("unable to check if AppArmor is supported by this system: %w", err)
		return
	}

	templateFileName := "static-files/" + apparmorProfName
	appArmorProfile, err := installationFiles.ReadFile(templateFileName)
	if err != nil {
		err = fmt.Errorf("unable to retrieve configuration file from embedded filesystem: %w", err)
		return
	}
	appArmorProfile = step.applyTemplateMacros(appArmorProfile)
	templateReader := bytes.NewReader(appArmorProfile)

	isEqual, err := fsops.FileEqualsReader(appArmorProfilePath, templateReader)
	if err != nil {
		return
	}
	if isEqual {
		ctx.logger.Verbose("AppArmor profile file '%s' is up-to-date", appArmorProfilePath)
		alreadyDone = true
		return
	}

	ctx.logger.Verbose("AppArmor profile file '%s' differs from embedded file", appArmorProfilePath)
	return
}

// If apparmor LSM is available on this system and running as root, auto install the profile - failures are not printed under normal verbosity
func (step *InstallAppArmorStep) Apply(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	ctx.logger.Verbose("Installing AppArmor profile file to '%s'", appArmorProfilePath)

	appArmorProfile, err := installationFiles.ReadFile("static-files/" + apparmorProfName)
	if err != nil {
		err = fmt.Errorf("unable to retrieve profile file from embedded filesystem: %w", err)
		return
	}
	appArmorProfile = step.applyTemplateMacros(appArmorProfile)

	step.backupProfileFile, step.backupCreated, err = fsops.MakeFileBackup(appArmorProfilePath)
	if err != nil {
		err = fmt.Errorf("failed to backup existing profile file: %w", err)
		return
	}

	// Write Apparmor Profile to /etc
	err = os.WriteFile(appArmorProfilePath, appArmorProfile, 0644)
	if err != nil {
		err = fmt.Errorf("failed to write apparmor profile: %w", err)
		return
	}
	step.installedNew = true

	// Enact Profile
	command := exec.Command("apparmor_parser", "-r", appArmorProfilePath)
	output, err := command.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("failed to reload apparmor profile: %w: %s", err, string(output))
		return
	}
	step.profileApplied = true

	ctx.logger.Success("Successfully installed AppArmor Profile")
	return
}

func (step *InstallAppArmorStep) Rollback(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Remove installed profile file if we installed one
	if step.installedNew {
		if step.profileApplied {
			// Remove profile
			command := exec.Command("apparmor_parser", "-R", appArmorProfilePath)
			output, err := command.CombinedOutput()
			if err != nil {
				ctx.logger.Error("failed to unload apparmor profile: %v: %s", err, string(output))
			}
		}

		err := os.Remove(appArmorProfilePath)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove AppArmor profile file: %v", err)
		}
	}

	// Previous a file at target path, move back into place
	if step.backupCreated {
		ctx.logger.Verbose("Restoring previous profile file from '%s' to '%s'", step.backupProfileFile, appArmorProfilePath)

		err := os.Rename(step.backupProfileFile, appArmorProfilePath)
		if err != nil {
			ctx.logger.Error("failed to restore backup: %v", err)
		} else {
			dir, err := os.Open(filepath.Dir(appArmorProfilePath))
			if err != nil {
				ctx.logger.Error("failed to open profile file directory: %v", err)
			} else {
				err = dir.Sync()
				if err != nil {
					ctx.logger.Error("failed to sync profile file directory: %v", err)
				}
				_ = dir.Close()
			}

			// Backup restored, reenact that profile
			command := exec.Command("apparmor_parser", "-r", appArmorProfilePath)
			output, err := command.CombinedOutput()
			if err != nil {
				ctx.logger.Error("failed to enact previous apparmor profile: %v: %s", err, string(output))
			}
		}
	}
}

func (step *InstallAppArmorStep) PostApply(ctx *context) {
	if step.backupCreated {
		err := os.Remove(step.backupProfileFile)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove previous AppArmor profile file backup: %v", err)
			return
		}
	}
}

func (step *InstallAppArmorStep) Uninstall(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Check if apparmor /sys path exists
	_, err = os.Stat(sysAAProfilePath)
	if os.IsNotExist(err) {
		err = nil
		return
	} else if err != nil {
		err = fmt.Errorf("unable to check if AppArmor is supported by this system: %w", err)
		return
	}

	// Warn about confined apparmor profile for uninstall
	if strings.Contains(os.Args[0], global.DefaultBinaryPath) {
		ctx.logger.Info("WARNING: uninstall will fail if calling this binary from within the apparmor profile\n")
		ctx.logger.Info("  Run this command and retry the uninstall: 'apparmor_parser -R %s'\n", appArmorProfilePath)
	}

	// Remove Profile
	command := exec.Command("apparmor_parser", "-R", appArmorProfilePath)
	output, err := command.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "not found, skipping") {
			err = fmt.Errorf("failed to disable apparmor profile: %w: %s", err, string(output))
			return
		}
	}

	// Remove Apparmor Profile File
	err = os.Remove(appArmorProfilePath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to remove apparmor profile: %w", err)
		return
	} else {
		err = nil
	}

	ctx.logger.Success("Successfully uninstalled AppArmor Profile")
	return
}
