package assembler

import (
	"context"
	"fmt"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard"
	"time"
)

// Create new shard+assembler
func (manager *Manager) AddInstance() (instanceID string) {
	// Only need to add/remove exactly one at a time
	manager.scalingMutex.Lock()
	defer manager.scalingMutex.Unlock()

	// Grab the next sequence for ID
	instanceID = fmt.Sprintf("%d", manager.nextInstanceID)
	_, ok := manager.routing.Load().instances[instanceID]
	if ok {
		// Instance already exists, no-op
		return
	}
	manager.nextInstanceID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, instanceID)
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Create new defrag instance
	shard := shard.New(logctx.GetTagList(manager.ctx), 1024, &manager.Config.PacketDeadline)
	instance := manager.newWorker(shard)

	// Create new context for both watcher/assembler
	workerCtx, cancelInstances := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, logctx.LoggerKey, logctx.GetLogger(manager.ctx))

	// Cancel for both instances
	instance.cancel = cancelInstances

	instance.wg.Add(2)
	go func() {
		// Run the deadline evaluator
		defer instance.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instance.Shard.Namespace)
		instance.Shard.StartTimeoutWatcher(workerCtx)
	}()
	go func() {
		// Run the assembler
		defer instance.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instance.namespace)
		instance.run(workerCtx)
	}()

	// Update routing view
	for {
		oldSnap := manager.routing.Load()

		newMap := make(map[string]*Instance)
		var newIDs []string

		if oldSnap != nil {
			for id, instance := range oldSnap.instances {
				newMap[id] = instance
				newIDs = append(newIDs, id)
			}
		}

		newMap[instanceID] = instance
		newIDs = append(newIDs, instanceID)

		newSnap := &routingSnapshot{
			instances: newMap,
			ids:       newIDs,
		}

		if manager.routing.CompareAndSwap(oldSnap, newSnap) {
			break
		}
	}
	return
}

// Removes the oldest shard+assembler instance.
// RemovedID will be empty when there are no more instances to remove
func (manager *Manager) RemoveOldestInstance() (removedID string) {
	ids := manager.routing.Load().ids
	if len(ids) == 0 {
		return
	}
	manager.removeInstance(ids[0])
	removedID = ids[0]
	return
}

// Gracefully shuts down and removes an instance from routing snapshot.
func (manager *Manager) removeInstance(instanceID string) {
	// Only need to add/remove exactly one at a time
	manager.scalingMutex.Lock()
	defer manager.scalingMutex.Unlock()

	oldSnap := manager.routing.Load()
	if oldSnap == nil {
		return
	}

	instance, exists := oldSnap.instances[instanceID]
	if !exists {
		return
	}

	if instance == nil {
		return
	}

	// Stop new bucket creation in this shard and wait for drain
	instance.Shard.InShutdown.Store(true)
	success, last := atomics.WaitUntilZero(&instance.Shard.Metrics.TotalBuckets, 15*time.Second) // Wait for buckets to fill or timeout
	if !success {
		logctx.LogStdWarn(manager.ctx,
			"assembler id %s: shard total buckets did not empty in time: dropped %d messages\n",
			instanceID, last)
	}

	success, last = atomics.WaitUntilZero(&instance.Shard.Metrics.WaitingBuckets, 15*time.Second) // Wait for assembler to pull last bucket
	if !success {
		logctx.LogStdWarn(manager.ctx,
			"assembler id %s: shard waiting buckets queue did not empty in time: dropped %d messages\n",
			instanceID, last)
	}

	if instance.cancel != nil {
		instance.cancel()
	}

	instance.wg.Wait()

	// Create new routing snapshot
	for {
		newMap := make(map[string]*Instance, len(oldSnap.instances)-1)
		newIDs := make([]string, 0, len(oldSnap.ids)-1)

		for id, instance := range oldSnap.instances {
			if id == instanceID {
				continue
			}
			newMap[id] = instance
		}
		for _, id := range oldSnap.ids {
			if id != instanceID {
				newIDs = append(newIDs, id)
			}
		}

		newSnap := &routingSnapshot{
			instances: newMap,
			ids:       newIDs,
		}

		if manager.routing.CompareAndSwap(oldSnap, newSnap) {
			break
		}
	}
}
