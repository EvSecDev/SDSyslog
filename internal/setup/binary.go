package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/fsops"
	"sdsyslog/internal/global"
)

type InstallBinaryStep struct {
	normalizedSourcePath string
	backupTgtPath        string
	replacedExisting     bool // Previous executable file existed at target path and was backed up
	installedNew         bool // New executable file was written to target path
}

func (step *InstallBinaryStep) Name() string {
	return "Executable"
}

func (step *InstallBinaryStep) NeedsApply(ctx *context) (alreadyInstalled bool, err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	selfPath, err := os.Executable()
	if err != nil {
		err = fmt.Errorf("unable to find path to current process: %w", err)
		return
	}

	ctx.logger.Verbose("Found self executable path '%s'", selfPath)

	selfPath, err = fsops.NormalizePath(selfPath)
	if err != nil {
		return
	}

	ctx.logger.Verbose("Normalized self executable path to '%s'", selfPath)
	step.normalizedSourcePath = selfPath

	equal, err := fsops.FilesEqual(selfPath, global.DefaultBinaryPath)
	if err != nil {
		err = fmt.Errorf("failed to check equality between files '%s' and '%s': %w", selfPath, global.DefaultBinaryPath, err)
		return
	}
	if equal {
		alreadyInstalled = true
		return
	}

	return
}

func (step *InstallBinaryStep) Apply(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	step.backupTgtPath, step.replacedExisting, err = fsops.MakeFileBackup(global.DefaultBinaryPath)
	if err != nil {
		err = fmt.Errorf("failed to backup existing executable: %w", err)
		return
	}

	err = fsops.AtomicFileReplace(step.normalizedSourcePath, global.DefaultBinaryPath, 0755)
	if err != nil {
		err = fmt.Errorf("failed to move self executable file to target executable path: %w", err)
		return
	}
	step.installedNew = true

	if ctx.mode == global.RecvMode {
		// Apply capabilities directly to binary file for receiver listener
		err = fsops.SetCapabilities(global.DefaultBinaryPath,
			fsops.CapEffective|fsops.CapInheritable|fsops.CapPermitted,
			fsops.CapSYSResource, fsops.CapBPF)
		if err != nil {
			err = fmt.Errorf("failed to set required Linux capabilities (for eBPF socket draining): %w", err)
			return
		}
		ctx.logger.Verbose("Set capabilities CAP_SYS_RESOURCE and CAP_BPF on executable file '%s'", global.DefaultBinaryPath)
	}

	ctx.logger.Success("Successfully installed executable to '%s'", global.DefaultBinaryPath)
	return
}

func (step *InstallBinaryStep) Rollback(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Remove installed binary if we installed one
	if step.installedNew {
		err := os.Remove(global.DefaultBinaryPath)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove target executable path: %v", err)
		}
	}

	// Previous a file at target path, move back into place
	if step.replacedExisting {
		ctx.logger.Verbose("Restoring previous executable from '%s' to '%s'", step.backupTgtPath, global.DefaultBinaryPath)

		err := os.Rename(step.backupTgtPath, global.DefaultBinaryPath)
		if err != nil {
			ctx.logger.Error("failed to restore backup: %v", err)
		} else {
			dir, err := os.Open(filepath.Dir(global.DefaultBinaryPath))
			if err != nil {
				ctx.logger.Error("failed to open target executable directory: %v", err)
			} else {
				err = dir.Sync()
				if err != nil {
					ctx.logger.Error("failed to sync target executable directory: %v", err)
				}
				_ = dir.Close()
			}
		}
	}
}

func (step *InstallBinaryStep) PostApply(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	if step.replacedExisting {
		err := os.Remove(step.backupTgtPath)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove previous executable file backup: %v", err)
			return
		}
	}

	if step.installedNew {
		// Remove current path once install completed successfully
		err := os.Remove(step.normalizedSourcePath)
		if err != nil && !os.IsNotExist(err) {
			ctx.logger.Error("failed to remove source executable file: %v", err)
			return
		}
	}
}

func (step *InstallBinaryStep) Uninstall(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	err = os.Remove(global.DefaultBinaryPath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to remove target executable path: %w", err)
		return
	} else {
		// File is gone
		err = nil
	}

	ctx.logger.Success("Successfully removed executable from '%s'", global.DefaultBinaryPath)
	return
}
