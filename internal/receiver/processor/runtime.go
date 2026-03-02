package processor

import (
	"context"
	"sdsyslog/internal/logctx"
	"strconv"
)

// Create additional ingest instance
func (manager *Manager) AddInstance() (id int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	id = int(manager.NextID)
	manager.NextID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(id))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	processor := newWorker(logctx.GetTagList(manager.ctx),
		manager.Inbox,
		manager.routingView,
		manager.Config.PastMsgCutoff,
		manager.Config.FutureMsgCutoff)

	manager.Instances[id] = processor

	// Create new context for both watcher/assembler
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	processor.cancel = cancelInstances
	ingestCtx = context.WithValue(ingestCtx, logctx.LoggerKey, logctx.GetLogger(manager.ctx))

	processor.wg.Add(1)
	go func() {
		defer processor.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, processor.namespace)
		processor.run(ingestCtx)
	}()
	return
}

// Remove existing instance
func (manager *Manager) RemoveInstance(id int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	processor, ok := manager.Instances[id]
	if ok {
		if processor.cancel != nil {
			processor.cancel()
		}

		processor.wg.Wait()

		delete(manager.Instances, id)
	}
}
