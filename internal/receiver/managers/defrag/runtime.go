package defrag

import (
	"context"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/shard"
	"strconv"
)

// Create new shard+assembler
func (manager *InstanceManager) AddInstance() (instanceID int) {
	// Lock manager for new spawn
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	// Grab the next sequence for ID
	instanceID = len(manager.InstancePairs) // always gets us the next index position

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(instanceID))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	// Create new defrag instance
	shard := shard.New(logctx.GetTagList(manager.ctx), 1024, &manager.PacketDeadline)
	InstancePair := &InstancePair{
		Shard:     shard,
		Assembler: assembler.New(logctx.GetTagList(manager.ctx), shard, manager.outQueue, manager.Routing),
	}

	manager.InstancePairs = append(manager.InstancePairs, InstancePair)

	// Create new context for both watcher/assembler
	workerCtx, cancelInstances := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	// Cancel for both instances
	InstancePair.cancel = cancelInstances

	InstancePair.wg.Add(2)
	go func() {
		// Run the deadline evaluator
		defer InstancePair.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, InstancePair.Shard.Namespace)
		InstancePair.Shard.StartTimeoutWatcher(workerCtx)
	}()
	go func() {
		// Run the assembler
		defer InstancePair.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, InstancePair.Assembler.Namespace)
		InstancePair.Assembler.Run(workerCtx)
	}()
	return
}

// Remove existing shard+assembler
func (manager *InstanceManager) RemoveInstance(instanceID int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	if instanceID < 0 || instanceID >= len(manager.InstancePairs) {
		return
	}

	instancePair := manager.InstancePairs[instanceID]
	if instancePair == nil {
		return
	}

	// Stop new bucket creation in this shard and wait for drain
	instancePair.Shard.InShutdown = true
	success, last := atomics.WaitUntilZero(&instancePair.Shard.Metrics.TotalBuckets) // Wait for buckets to fill or timeout
	if !success {
		logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.WarnLog,
			"assembler id %d: shard total buckets did not empty in time: dropped %d messages\n", instanceID, last)
	}

	success, last = atomics.WaitUntilZero(&instancePair.Shard.Metrics.WaitingBuckets) // Wait for assembler to pull last bucket
	if !success {
		logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.WarnLog,
			"assembler id %d: shard waiting buckets queue did not empty in time: dropped %d messages\n", instanceID, last)
	}

	if instancePair.cancel != nil {
		instancePair.cancel()
	}

	instancePair.wg.Wait()

	// Collapse slice
	manager.InstancePairs = append(manager.InstancePairs[:instanceID], manager.InstancePairs[instanceID+1:]...)
}
