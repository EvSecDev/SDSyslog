package ingest

import (
	"context"
	"fmt"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
)

// Create journal ingest instance
func (manager *InstanceManager) AddJrnlInstance(stateFile string) (err error) {
	if manager.JournalSource != nil {
		err = fmt.Errorf("cannot start a new journal instance with one running")
		return
	}

	manager.ctx = logctx.AppendCtxTag(manager.ctx, global.NSoJrnl)
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	new, err := journald.NewInput(logctx.GetTagList(manager.ctx), stateFile, manager.outQueue)
	if err != nil {
		return
	}

	// Create new context
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestCtx = context.WithValue(ingestCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	// Worker for local journal
	ingestInstance := &JrnlWorker{
		Worker: new,
		cancel: cancelInstances,
	}
	manager.JournalSource = ingestInstance

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.Worker.Namespace)
		ingestInstance.Worker.Reader(ingestCtx)
	}()

	err = new.Start()
	if err != nil {
		return
	}
	err = new.CheckError()
	if err != nil {
		return
	}

	return
}

// Remove existing journal ingest instance
func (manager *InstanceManager) RemoveJrnlInstance() (err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	if manager.JournalSource.cancel != nil {
		manager.JournalSource.cancel()
	}
	manager.JournalSource.wg.Wait()
	err = manager.JournalSource.Worker.Shutdown()
	return
}
