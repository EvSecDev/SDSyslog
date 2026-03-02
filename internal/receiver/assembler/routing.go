package assembler

import (
	"sdsyslog/internal/receiver/shard"
)

// Returns a shallow copy of all instance pairs in current routing snapshot
func (rs *RoutingState) GetInstancePairs() (instances map[string]*Instance) {
	currentInstances := rs.manager.routing.Load().instances
	instances = make(map[string]*Instance, len(currentInstances))

	// Make a map copy - but preserve the pair pointer value
	for id, instance := range currentInstances {
		instances[id] = instance
	}
	return
}

// Returns FIFO-ordered list of instance IDs in current routing snapshot
func (rs *RoutingState) GetAllIDs() (shardIDs []string) {
	ids := rs.manager.routing.Load().ids
	shardIDs = make([]string, len(ids))
	copy(shardIDs, ids)
	return
}

// Retrieves list of non-draining instance shard identifiers
func (rs *RoutingState) GetNonDrainingIDs() (availShardIDs []string) {
	instances := rs.manager.routing.Load().instances
	for id, instance := range instances {
		instance.Shard.Mu.Lock()
		if instance.Shard.InShutdown.Load() {
			instance.Shard.Mu.Unlock()
			continue
		}
		instance.Shard.Mu.Unlock()

		availShardIDs = append(availShardIDs, id)
	}
	return
}

// Checks if shard contains a particular bucket
func (rs *RoutingState) BucketExists(shardID string, bucketKey string) (present bool) {
	instances := rs.manager.routing.Load().instances
	instance, ok := instances[shardID]
	if !ok {
		return
	}
	shard := instance.Shard

	shard.Mu.Lock()
	defer shard.Mu.Unlock()
	_, present = shard.Buckets[bucketKey]
	return
}

// Checks if any shard contains the specified bucket.
// Returns true if any active shard contains the bucket.
func (rs *RoutingState) BucketExistsAnywhere(bucketKey string) (present bool) {
	instances := rs.manager.routing.Load().instances
	for id := range instances {
		present = rs.BucketExists(id, bucketKey)
		if present {
			return
		}
	}
	return
}

// Retrieves instance pointer for a particular index
func (rs *RoutingState) GetShard(shardID string) (shardInst *shard.Instance) {
	instances := rs.manager.routing.Load().instances
	instance, ok := instances[shardID]
	if !ok {
		return
	}
	shardInst = instance.Shard
	return
}

// Checks if particular instance has been marked as shutdown
func (rs *RoutingState) IsShardShutdown(shardID string) (shutdown bool) {
	instances := rs.manager.routing.Load().instances
	instance, ok := instances[shardID]
	if !ok {
		shutdown = true
		return
	}
	shardInst := instance.Shard
	shutdown = shardInst.InShutdown.Load()
	return
}

// Checks if the local FIPR receiver go routine has been started
func (rs *RoutingState) IsFIPRRunning() (running bool) {
	running = rs.manager.FIPRRunning.Load()
	return
}
