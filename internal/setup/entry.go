// Handles all installation/setup/configuration/updates
package setup

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Read in installation static files at compile time
//
//go:embed static-files/*
var installationFiles embed.FS

func NewInstaller(mode string, suiteID uint8, dryRun bool, verbose bool) (inst *Installer, err error) {
	// Must run as root
	if os.Geteuid() != 0 {
		err = fmt.Errorf("installation must be run as root. Please execute with elevated privileges")
		return
	}

	inst = &Installer{
		ctx: &context{
			mode:    mode,
			logger:  newLogger(verbose),
			dryRun:  dryRun,
			suiteID: suiteID,
		},
		steps: []Step{
			&InstallBinaryStep{},
			&InstallAutocompleteStep{},
			&InstallStateStep{},
			&InstallConfigStep{},
			&InstallAppArmorStep{},
			&InstallSystemdStep{},
		},
	}
	return
}

func (inst *Installer) RunInstall() (err error) {
	var completed []Step
	log := inst.ctx.logger

	log.Info("Running installation")

	for _, step := range inst.steps {
		log.Step(step.Name())
		log.Indent()

		var done bool
		done, err = step.NeedsApply(inst.ctx)
		if err != nil {
			err = fmt.Errorf("%s check failed: %w", step.Name(), err)
			return
		}

		if done {
			log.Info("Already installed and up-to-date")
			log.Dedent()
			continue
		}

		if inst.ctx.dryRun {
			log.Info("Dry-run enabled, skipping application of changes for %s", step.Name())
			log.Dedent()
			continue
		}

		err = step.Apply(inst.ctx)
		if err != nil {
			// rollback this step (not in completed list)
			step.Rollback(inst.ctx)

			// rollback completed steps in reverse
			for j := len(completed) - 1; j >= 0; j-- {
				completed[j].Rollback(inst.ctx)
			}
			err = fmt.Errorf("%s failed install: %w", step.Name(), err)
			log.Dedent()
			return
		}

		completed = append(completed, step)
		log.Dedent()
	}

	// Run post-full-success actions
	for _, step := range inst.steps {
		if inst.ctx.dryRun {
			continue
		}

		step.PostApply(inst.ctx)
	}

	if inst.ctx.dryRun {
		log.Success("Installation dry-run completed successfully")
	} else {
		log.Success("Installation completed successfully")
	}
	return
}

func (inst *Installer) RunUninstall() (err error) {
	// Only ask if in terminal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		// File exists, prompt user for confirmation to overwrite
		fmt.Printf("Are you SURE you want to uninstall? (this will remove the configuration files) (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		var input string
		input, err = reader.ReadString('\n')
		if err != nil {
			return
		}
		input = strings.TrimSpace(input)

		if strings.ToLower(input) != "yes" {
			fmt.Printf("Aborting uninstall\n")
			return
		}
	}

	log := inst.ctx.logger

	log.Info("Running uninstallation")

	var failureCount int
	for j := len(inst.steps) - 1; j >= 0; j-- {
		step := inst.steps[j]
		log.Step(step.Name())
		log.Indent()

		if inst.ctx.dryRun {
			log.Info("Dry-run enabled, skipping application of removals for %s", step.Name())
			log.Dedent()
			continue
		}

		err = step.Uninstall(inst.ctx)
		if err != nil {
			log.Error("%s failed uninstall: %v", step.Name(), err)
			log.Dedent()
			failureCount++
			continue
		}

		log.Dedent()
	}

	if inst.ctx.dryRun && failureCount > 0 {
		log.Info("Uninstall dry-run completed with problems")
	} else if inst.ctx.dryRun && failureCount == 0 {
		log.Success("Uninstall dry-run completed successfully")
	} else if failureCount > 0 {
		log.Info("Uninstall completed with problems")
	} else if failureCount == 0 {
		log.Success("Uninstall completed successfully")
	}
	return
}
