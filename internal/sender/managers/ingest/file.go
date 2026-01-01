package ingest

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender/listener"
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

	// Create unique state file for this source
	stateFileDir := filepath.Dir(stateFile)
	stateFileName := filepath.Base(stateFile)

	newStateFileName := base64.RawURLEncoding.EncodeToString([]byte(filePath)) + "_" + stateFileName // Using full file path as prefix to state file
	newStateFile := filepath.Join(stateFileDir, newStateFileName)

	// Worker for this file
	ingestInstance := &FileWorker{
		Worker: listener.NewFileSource(logctx.GetTagList(manager.ctx), filePath, newStateFile, manager.outQueue),
	}

	manager.ctx = logctx.AppendCtxTag(manager.ctx, filename)
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Create new context
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestCtx = context.WithValue(ingestCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))
	manager.FileSources[filePath] = ingestInstance
	ingestInstance.cancel = cancelInstances

	// Open file
	ingestInstance.Worker.Source, err = os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.ErrorLog, "failed to open source file: %v\n", err)
		return
	}

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

	if fileSource.Worker.Source != nil {
		fileSource.Worker.Source.Close()
	}
	if fileSource.cancel != nil {
		fileSource.cancel()
	}
	fileSource.wg.Wait()
	manager.FileSources[filename] = nil
	return
}
