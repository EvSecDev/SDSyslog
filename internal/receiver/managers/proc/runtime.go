package proc

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/processor"
	"strconv"
)

// Create additional ingest instance
func (manager *InstanceManager) AddInstance() (id int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	id = manager.nextID
	manager.nextID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(id))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	ingestInstance := &Instance{
		Processor: processor.New(logctx.GetTagList(manager.ctx), manager.Inbox, manager.routingView),
	}

	manager.Instances[id] = ingestInstance

	// Create new context for both watcher/assembler
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestInstance.cancel = cancelInstances
	ingestCtx = context.WithValue(ingestCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.Processor.Namespace)
		ingestInstance.Processor.Run(ingestCtx)
	}()
	return
}

// Remove existing instance
func (manager *InstanceManager) RemoveInstance(id int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	ingestInstance, ok := manager.Instances[id]
	if ok {
		if ingestInstance.cancel != nil {
			ingestInstance.cancel()
		}

		ingestInstance.wg.Wait()

		delete(manager.Instances, id)
	}
}
