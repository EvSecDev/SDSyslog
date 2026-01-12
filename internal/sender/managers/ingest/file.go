package ingest

import (
	"context"
	"fmt"
	"path/filepath"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
)

// Create file ingest instance
func (manager *InstanceManager) AddFileInstance(filePath string, stateFile string) (err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	filename := filepath.Base(filePath)

	_, ok := manager.FileSources[filename]
	if ok {
		err = fmt.Errorf("cannot start a new file instance with one running for path '%s'", filePath)
		return
	}

	manager.ctx = logctx.AppendCtxTag(manager.ctx, filename)
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Worker for this file
	ingestInstance := &FileWorker{}
	ingestInstance.Worker, err = file.NewInput(logctx.GetTagList(manager.ctx), filePath, stateFile, manager.outQueue)
	if err != nil {
		return
	}

	// Create new context
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestCtx = context.WithValue(ingestCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))
	manager.FileSources[filePath] = ingestInstance
	ingestInstance.cancel = cancelInstances

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.Worker.Namespace)
		ingestInstance.Worker.Run(ingestCtx)
	}()
	return
}

// Remove existing file ingest instance
func (manager *InstanceManager) RemoveFileInstance(filename string) (err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	fileSource, ok := manager.FileSources[filename]
	if !ok {
		err = fmt.Errorf("no file source for '%s'", filename)
		return
	}

	err = fileSource.Worker.Shutdown()
	if err != nil {
		return
	}
	if fileSource.cancel != nil {
		fileSource.cancel()
	}
	fileSource.wg.Wait()
	manager.FileSources[filename] = nil
	return
}
