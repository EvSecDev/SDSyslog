package output

import (
	"context"
	"fmt"
	"sdsyslog/internal/externalio/beats"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/logctx"
)

// Create and start new output instance
func (manager *Manager) AddInstance(filePath string, journaldURL string, beatsAddress string) (err error) {
	if filePath == "" && journaldURL == "" && beatsAddress == "" {
		err = fmt.Errorf("no outputs enabled/configured")
		return
	}

	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, logctx.LoggerKey, logctx.GetLogger(manager.ctx))

	manager.cancel = cancelInstance
	manager.Instance = *manager.newWorker()

	// Add outputs
	manager.Instance.fileMod, err = file.NewOutput(filePath)
	if err != nil {
		return
	}
	manager.Instance.jrnlMod, err = journald.NewOutput(journaldURL)
	if err != nil {
		return
	}
	manager.Instance.beatsMod, err = beats.NewOutput(beatsAddress)
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
func (manager *Manager) RemoveInstance() {
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
}
