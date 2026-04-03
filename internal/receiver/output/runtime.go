package output

import (
	"context"
	"sdsyslog/internal/iomodules/beats"
	"sdsyslog/internal/iomodules/dbusnotify"
	"sdsyslog/internal/iomodules/file"
	"sdsyslog/internal/iomodules/generic"
	"sdsyslog/internal/iomodules/journald"
	"sdsyslog/internal/logctx"
)

// Create and start new output instance
func (manager *Manager) AddWorkers() (err error) {
	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(manager.ctx)

	manager.cancel = cancelInstance
	manager.Instance = *manager.newWorker()

	const defaultFileBatchSize int = 20

	// Add outputs
	manager.Instance.fileMod, err = file.NewOutput(manager.Config.FilePath, defaultFileBatchSize)
	if err != nil {
		return
	}
	manager.Instance.jrnlMod, err = journald.NewOutput(manager.Config.JournaldURL)
	if err != nil {
		return
	}
	manager.Instance.beatsMod, err = beats.NewOutput(manager.Config.BeatsAddress)
	if err != nil {
		return
	}
	manager.Instance.rawMod = generic.NewOutput(manager.Config.RawWriter)
	manager.Instance.DBUSnotify, err = dbusnotify.NewOutput(manager.Config.EnableDBUSNotify)
	if err != nil {
		return
	}

	// Start worker
	manager.wg.Add(1)
	go func() {
		defer manager.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, manager.Instance.namespace)
		manager.Instance.run(workerCtx)
	}()
	return
}

// Shutdown existing file output instance
func (manager *Manager) RemoveWorkers() {
	if manager.cancel != nil {
		manager.cancel()
	}
	manager.wg.Wait()

	err := manager.Instance.fileMod.Shutdown()
	if err != nil {
		logctx.LogStdErr(manager.ctx,
			"failed to shutdown file module: %w\n", err)
	}
	err = manager.Instance.jrnlMod.Shutdown()
	if err != nil {
		logctx.LogStdErr(manager.ctx,
			"failed to shutdown journal module: %w\n", err)
	}
	err = manager.Instance.beatsMod.Shutdown()
	if err != nil {
		logctx.LogStdErr(manager.ctx,
			"failed to shutdown beats module: %w\n", err)
	}
	err = manager.Instance.rawMod.Shutdown()
	if err != nil {
		logctx.LogStdErr(manager.ctx,
			"failed to shutdown raw module: %w\n", err)
	}
	err = manager.Instance.DBUSnotify.Shutdown()
	if err != nil {
		logctx.LogStdErr(manager.ctx,
			"failed to shutdown DBUS notify module: %w\n", err)
	}
}
