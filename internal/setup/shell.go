package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/global"
)

type InstallAutocompleteStep struct {
	autoCompleteFilePath string
	created              bool
}

func (step *InstallAutocompleteStep) Name() string {
	return "Shell Autocomplete"
}

func (step *InstallAutocompleteStep) NeedsApply(ctx *context) (alreadyDone bool, err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	step.autoCompleteFilePath = filepath.Join(sysAutocompleteDir, global.ProgBaseName)

	_, err = os.Stat(step.autoCompleteFilePath)
	if err == nil {
		ctx.logger.Verbose("autocomplete already exists: %s", step.autoCompleteFilePath)
		alreadyDone = true
		return
	}

	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to stat autocomplete file: %w", err)
		return
	} else if err != nil && os.IsNotExist(err) {
		ctx.logger.Verbose("Autocomplete script '%s' not present", step.autoCompleteFilePath)
		err = nil
	}

	return
}

func (step *InstallAutocompleteStep) Apply(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	ctx.logger.Verbose("Installing autocompletion script to '%s'", step.autoCompleteFilePath)

	autoCompleteFunc, err := installationFiles.ReadFile("static-files/autocomplete.sh")
	if err != nil {
		err = fmt.Errorf("unable to retrieve autocomplete file from embedded filesystem: %w", err)
		return
	}

	_, err = os.Stat(sysAutocompleteDir)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to stat system autocomplete directory: %w", err)
		return
	}

	step.autoCompleteFilePath = filepath.Join(sysAutocompleteDir, global.ProgBaseName)
	err = os.WriteFile(step.autoCompleteFilePath, autoCompleteFunc, 0644)
	if err != nil {
		err = fmt.Errorf("failed to write autocompletion file: %w", err)
		return
	}
	step.created = true

	ctx.logger.Success("Successfully wrote autocompletion script to '%s'", step.autoCompleteFilePath)
	return
}

func (step *InstallAutocompleteStep) Rollback(ctx *context) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	if !step.created {
		return
	}

	err := os.Remove(step.autoCompleteFilePath)
	if err != nil && !os.IsNotExist(err) {
		ctx.logger.Error("failed to remove autocomplete file %s: %v", step.autoCompleteFilePath, err)
		return
	}

	ctx.logger.Verbose("removed autocomplete file %s", step.autoCompleteFilePath)
}

func (step *InstallAutocompleteStep) PostApply(ctx *context) {
	// No-op
}

func (step *InstallAutocompleteStep) Uninstall(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	autoCompleteFilePath := filepath.Join(sysAutocompleteDir, global.ProgBaseName)

	_, err = os.Stat(autoCompleteFilePath)
	if err != nil && !os.IsNotExist(err) {
		// Unable to stat
		err = fmt.Errorf("failed to stat system autocomplete file: %w", err)
		return
	} else if err != nil && os.IsNotExist(err) {
		// File already removed
		err = nil
	} else {
		// File present
		err = os.Remove(autoCompleteFilePath)
		if err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("failed to remove autocompletion file: %w", err)
			return
		}
	}

	ctx.logger.Success("Successfully removed shell autocompletion")
	return
}
