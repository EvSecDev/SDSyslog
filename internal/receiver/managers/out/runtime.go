package out

import (
	"context"
	"fmt"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/output"
)

// Create and start new output instance
func (manager *InstanceManager) AddInstance(filePath string, journaldURL string) (err error) {
	if filePath == "" && journaldURL == "" {
		err = fmt.Errorf("no outputs enabled/configured")
		return
	}

	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	instance := &OutputInstance{
		Worker: output.New(logctx.GetTagList(manager.ctx), manager.Queue),
		cancel: cancelInstance,
	}

	manager.Instance = instance

	// Add outputs
	instance.Worker.FileMod, err = file.NewOutput(filePath)
	if err != nil {
		return
	}
	instance.Worker.JrnlMod, err = journald.NewOutput(journaldURL)
	if err != nil {
		return
	}

	// Start worker
	instance.wg.Add(1)
	go func() {
		defer instance.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instance.Worker.Namespace)
		instance.Worker.Run(workerCtx)
	}()
	return
}

// Shutdown existing file output instance
func (manager *InstanceManager) RemoveInstance() {
	if manager.Instance == nil {
		return
	}
	if manager.Instance.cancel != nil {
		manager.Instance.cancel()
	}
	manager.Instance.wg.Wait()

	manager.Instance.Worker.FileMod.Shutdown()
	manager.Instance.Worker.JrnlMod.Shutdown()
}
