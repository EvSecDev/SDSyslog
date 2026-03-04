package ingest

import (
	"context"
	"fmt"
	"path/filepath"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/logctx"
)

// Create file ingest instance
func (manager *Manager) AddFileInstance(filePath string, stateFile string) (err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	filename := filepath.Base(filePath)

	_, ok := manager.FileSources[filename]
	if ok {
		err = fmt.Errorf("cannot start a new file instance with one running for path '%s'", filePath)
		return
	}

	manager.ctx = logctx.AppendCtxTag(manager.ctx, logctx.NSoFile)
	manager.ctx = logctx.AppendCtxTag(manager.ctx, filename)
	defer func() {
		manager.ctx = logctx.RemoveLastCtxTag(manager.ctx)
		manager.ctx = logctx.RemoveLastCtxTag(manager.ctx)
	}()

	// Worker for this file
	ingestInstance := &FileWorker{}
	ingestInstance.Module, err = file.NewInput(logctx.GetTagList(manager.ctx), filePath, stateFile, manager.outQueue)
	if err != nil {
		return
	}

	// Create new context
	ingestCtx, cancelInstances := context.WithCancel(manager.ctx)
	manager.FileSources[filePath] = ingestInstance
	ingestInstance.cancel = cancelInstances

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.Module.Namespace)
		ingestInstance.Module.Reader(ingestCtx)
	}()
	return
}

// Remove existing file ingest instance
func (manager *Manager) RemoveFileInstance(filename string) (err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	fileSource, ok := manager.FileSources[filename]
	if !ok {
		err = fmt.Errorf("no file source for '%s'", filename)
		return
	}

	err = fileSource.Module.Shutdown()
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
