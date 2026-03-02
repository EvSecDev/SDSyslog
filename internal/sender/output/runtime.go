package output

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
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(instanceID))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Create new worker instance
	newWorker := manager.newWorker()

	manager.Instances[instanceID] = newWorker

	// Create new context
	workerCtx, cancelInstances := context.WithCancel(context.Background())
	newWorker.cancel = cancelInstances
	workerCtx = context.WithValue(workerCtx, logctx.LoggerKey, logctx.GetLogger(manager.ctx))

	newWorker.wg.Add(1)
	go func() {
		// Run the worker
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
