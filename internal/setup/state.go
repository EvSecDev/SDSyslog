package setup

import (
	"os"
	"sdsyslog/internal/global"
)

type InstallStateStep struct{}

func (step *InstallStateStep) Name() string {
	return "State Keeping"
}

func (step *InstallStateStep) NeedsApply(ctx *context) (alreadyDone bool, err error) {
	// No-op
	alreadyDone = true
	return
}

func (step *InstallStateStep) Apply(ctx *context) (err error) {
	// No-op
	return
}

func (step *InstallStateStep) Rollback(ctx *context) {
	// No-op
}

func (step *InstallStateStep) PostApply(ctx *context) {
	// No-op
}

func (step *InstallStateStep) Uninstall(ctx *context) (err error) {
	ctx.logger.Indent()
	defer ctx.logger.Dedent()

	// Cleanup state dir
	err = os.RemoveAll(global.DefaultStateDir)
	if err != nil {
		ctx.logger.Error("failed removing state dir: %v", err)
		return
	}

	ctx.logger.Verbose("Removed state directory '%s'", global.DefaultStateDir)
	return
}
