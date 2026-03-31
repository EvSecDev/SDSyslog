package output

import (
	"context"
	"fmt"
	"io"
	"sdsyslog/internal/iomodules/beats"
	"sdsyslog/internal/iomodules/file"
	"sdsyslog/internal/iomodules/generic"
	"sdsyslog/internal/iomodules/journald"
	"sdsyslog/internal/logctx"
)

// Create and start new output instance
func (manager *Manager) AddInstance(filePath string, journaldURL string, beatsAddress string, rawWriter io.WriteCloser) (err error) {
	if filePath == "" && journaldURL == "" && beatsAddress == "" {
		err = fmt.Errorf("no outputs enabled/configured")
		return
	}

	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(manager.ctx)

	manager.cancel = cancelInstance
	manager.Instance = *manager.newWorker()

	const defaultFileBatchSize int = 20

	// Add outputs
	manager.Instance.fileMod, err = file.NewOutput(filePath, defaultFileBatchSize)
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
	manager.Instance.rawMod = generic.NewOutput(rawWriter, 0)

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
	err = manager.Instance.rawMod.Shutdown()
	if err != nil {
		logctx.LogStdErr(manager.ctx,
			"failed to shutdown raw module: %w\n", err)
	}
}
