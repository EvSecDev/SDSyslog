package out

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender/output"
	"strconv"
)

// Create new packaging instance
func (manager *InstanceManager) AddInstance() (instanceID int) {
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
	newWorker := &Instance{
		id:     instanceID,
		Worker: output.New(logctx.GetTagList(manager.ctx), manager.InQueue, manager.OutDest),
	}

	manager.Instances[instanceID] = newWorker

	// Create new context
	workerCtx, cancelInstances := context.WithCancel(context.Background())
	newWorker.cancel = cancelInstances
	workerCtx = context.WithValue(workerCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	newWorker.wg.Add(1)
	go func() {
		// Run the worker
		defer newWorker.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, newWorker.Worker.Namespace)
		newWorker.Worker.Run(workerCtx)
	}()
	return
}

// Remove existing packaging instance
func (manager *InstanceManager) RemoveInstance(instanceID int) {
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
