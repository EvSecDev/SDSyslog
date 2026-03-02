package assembler

import (
	"context"
	"sdsyslog/internal/logctx"
	"strconv"
)

// Create new packaging instance
func (manager *Manager) AddInstance() (instanceID int) {
	// Lock manager for new spawn
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	// Grab the next sequence for ID
	instanceID = manager.nextID
	manager.nextID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, logctx.NSmPack)
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(instanceID))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Create new worker instance
	newWorker := newWorker(logctx.GetTagList(manager.ctx),
		manager.InQueue,
		manager.outQueue,
		manager.Config.HostID,
		manager.Config.MaxPayloadSize)

	manager.Instances[instanceID] = newWorker

	// Create new context
	workerCtx, cancelInstances := context.WithCancel(context.Background())
	newWorker.cancel = cancelInstances
	workerCtx = context.WithValue(workerCtx, logctx.LoggerKey, logctx.GetLogger(manager.ctx))

	newWorker.wg.Add(1)
	go func() {
		// Run the assembler
		defer newWorker.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, newWorker.namespace)
		newWorker.run(workerCtx)
	}()
	return
}

// Remove existing packaging instance
func (manager *Manager) RemoveInstance(instanceID int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	instancePair, ok := manager.Instances[instanceID]
	if ok {
		if instancePair.cancel != nil {
			instancePair.cancel()
		}
		instancePair.wg.Wait()
		delete(manager.Instances, instanceID)
	}
}
