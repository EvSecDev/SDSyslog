package output

import (
	"context"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/iomodules/beats"
	"sdsyslog/internal/iomodules/dbusnotify"
	"sdsyslog/internal/iomodules/file"
	"sdsyslog/internal/iomodules/generic"
	"sdsyslog/internal/iomodules/journald"
	"sdsyslog/internal/logctx"
	"time"
)

// Create and start new output instance
func (manager *Manager) AddWorkers() (err error) {
	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(manager.ctx)

	manager.cancel = cancelInstance
	manager.Instance = *manager.newWorker()

	const defaultFileBatchSize int = 20
	const beatsRetryWriteCnt int = 6
	const beatsStartupRetryTime time.Duration = 5 * time.Minute

	// Add outputs from desired output modules (consume model only - modules do not register themselves)
	manager.Instance.outModules = make(map[string]iomodules.Output)
	fileMod, err := file.NewOutput(manager.Config.FilePath, defaultFileBatchSize)
	if err != nil {
		return
	}
	jrnlMod, err := journald.NewOutput(manager.Config.JournaldURL)
	if err != nil {
		return
	}
	beatsMod, err := beats.NewOutput(manager.Config.BeatsAddress, beatsRetryWriteCnt, beatsStartupRetryTime)
	if err != nil {
		return
	}
	rawMod := generic.NewOutput(manager.Config.RawWriter)
	dbusNotify, err := dbusnotify.NewOutput(manager.Config.EnableDBUSNotify)
	if err != nil {
		return
	}

	manager.Instance.outModules["file"] = fileMod
	manager.Instance.outModules["journald"] = jrnlMod
	manager.Instance.outModules["beats"] = beatsMod
	manager.Instance.outModules["raw"] = rawMod
	manager.Instance.outModules["notify"] = dbusNotify
	manager.Instance.initializeMetrics()

	// Start worker
	manager.wg.Go(func() {
		workerCtx := logctx.OverwriteCtxTag(workerCtx, manager.Instance.namespace)
		manager.Instance.run(workerCtx)
	})
	return
}

// Shutdown existing file output instance
func (manager *Manager) RemoveWorkers() {
	if manager.cancel != nil {
		manager.cancel()
	}

	shutdownTimeout := 10 * time.Second
	done := make(chan struct{})
	go func() {
		manager.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		logctx.LogStdErr(manager.ctx,
			"output instance did not exit within %s of cancel\n", shutdownTimeout.String())
	}

	if manager.Instance.outModules == nil {
		return
	}

	for moduleName, module := range manager.Instance.outModules {
		err := module.Shutdown()
		if err != nil {
			logctx.LogStdErr(manager.ctx,
				"failed to shutdown %s module: %w\n", moduleName, err)
		}
	}
}
