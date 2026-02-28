package defrag

import (
	"context"
	"fmt"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/shard"
	"time"
)

// Create new shard+assembler
func (manager *InstanceManager) AddInstance() (instanceID string) {
	// Only need to add/remove exactly one at a time
	manager.scalingMutex.Lock()
	defer manager.scalingMutex.Unlock()

	// Grab the next sequence for ID
	instanceID = fmt.Sprintf("%d", manager.nextInstanceID)
	_, ok := manager.routing.Load().pairs[instanceID]
	if ok {
		// Instance already exists, no-op
		return
	}
	manager.nextInstanceID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, instanceID)
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Create new defrag instance
	shard := shard.New(logctx.GetTagList(manager.ctx), 1024, &manager.PacketDeadline)
	instancePair := &InstancePair{
		Shard:     shard,
		Assembler: assembler.New(logctx.GetTagList(manager.ctx), shard, manager.outQueue),
	}

	// Create new context for both watcher/assembler
	workerCtx, cancelInstances := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	// Cancel for both instances
	instancePair.cancel = cancelInstances

	instancePair.wg.Add(2)
	go func() {
		// Run the deadline evaluator
		defer instancePair.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instancePair.Shard.Namespace)
		instancePair.Shard.StartTimeoutWatcher(workerCtx)
	}()
	go func() {
		// Run the assembler
		defer instancePair.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instancePair.Assembler.Namespace)
		instancePair.Assembler.Run(workerCtx)
	}()

	// Update routing view
	for {
		oldSnap := manager.routing.Load()

		newMap := make(map[string]*InstancePair)
		var newIDs []string

		if oldSnap != nil {
			for id, pair := range oldSnap.pairs {
				newMap[id] = pair
				newIDs = append(newIDs, id)
			}
		}

		newMap[instanceID] = instancePair
		newIDs = append(newIDs, instanceID)

		newSnap := &routingSnapshot{
			pairs: newMap,
			ids:   newIDs,
		}

		if manager.routing.CompareAndSwap(oldSnap, newSnap) {
			break
		}
	}
	return
}

// Removes the oldest shard+assembler instance.
// RemovedID will be empty when there are no more instances to remove
func (manager *InstanceManager) RemoveOldestInstance() (removedID string) {
	ids := manager.routing.Load().ids
	if len(ids) == 0 {
		return
	}
	manager.removeInstance(ids[0])
	removedID = ids[0]
	return
}

// Gracefully shuts down and removes an instance from routing snapshot.
func (manager *InstanceManager) removeInstance(instanceID string) {
	// Only need to add/remove exactly one at a time
	manager.scalingMutex.Lock()
	defer manager.scalingMutex.Unlock()

	oldSnap := manager.routing.Load()
	if oldSnap == nil {
		return
	}

	instancePair, exists := oldSnap.pairs[instanceID]
	if !exists {
		return
	}

	if instancePair == nil {
		return
	}

	// Stop new bucket creation in this shard and wait for drain
	instancePair.Shard.InShutdown.Store(true)
	success, last := atomics.WaitUntilZero(&instancePair.Shard.Metrics.TotalBuckets, 15*time.Second) // Wait for buckets to fill or timeout
	if !success {
		logctx.LogStdWarn(manager.ctx,
			"assembler id %s: shard total buckets did not empty in time: dropped %d messages\n",
			instanceID, last)
	}

	success, last = atomics.WaitUntilZero(&instancePair.Shard.Metrics.WaitingBuckets, 15*time.Second) // Wait for assembler to pull last bucket
	if !success {
		logctx.LogStdWarn(manager.ctx,
			"assembler id %s: shard waiting buckets queue did not empty in time: dropped %d messages\n",
			instanceID, last)
	}

	if instancePair.cancel != nil {
		instancePair.cancel()
	}

	instancePair.wg.Wait()

	// Create new routing snapshot
	for {
		newMap := make(map[string]*InstancePair, len(oldSnap.pairs)-1)
		newIDs := make([]string, 0, len(oldSnap.ids)-1)

		for id, pair := range oldSnap.pairs {
			if id == instanceID {
				continue
			}
			newMap[id] = pair
		}
		for _, id := range oldSnap.ids {
			if id != instanceID {
				newIDs = append(newIDs, id)
			}
		}

		newSnap := &routingSnapshot{
			pairs: newMap,
			ids:   newIDs,
		}

		if manager.routing.CompareAndSwap(oldSnap, newSnap) {
			break
		}
	}
}
