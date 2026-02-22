package defrag

import (
	"sdsyslog/internal/receiver/shard"
)

// Returns a shallow copy of all instance pairs in current routing snapshot
func (rs *RoutingState) GetInstancePairs() (pairs map[string]*InstancePair) {
	currentPairs := rs.manager.routing.Load().pairs
	pairs = make(map[string]*InstancePair, len(currentPairs))

	// Make a map copy - but preserve the pair pointer value
	for id, pair := range currentPairs {
		pairs[id] = pair
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
	instancePairs := rs.manager.routing.Load().pairs
	for id, pair := range instancePairs {
		pair.Shard.Mu.Lock()
		if pair.Shard.InShutdown.Load() {
			pair.Shard.Mu.Unlock()
			continue
		}
		pair.Shard.Mu.Unlock()

		availShardIDs = append(availShardIDs, id)
	}
	return
}

// Checks if shard contains a particular bucket
func (rs *RoutingState) BucketExists(shardID string, bucketKey string) (present bool) {
	instancePairs := rs.manager.routing.Load().pairs
	pair, ok := instancePairs[shardID]
	if !ok {
		return
	}
	shard := pair.Shard

	shard.Mu.Lock()
	defer shard.Mu.Unlock()
	_, present = shard.Buckets[bucketKey]
	return
}

// Checks if any shard contains the specified bucket.
// Returns true if any active shard contains the bucket.
func (rs *RoutingState) BucketExistsAnywhere(bucketKey string) (present bool) {
	instancePairs := rs.manager.routing.Load().pairs
	for id := range instancePairs {
		present = rs.BucketExists(id, bucketKey)
		if present {
			return
		}
	}
	return
}

// Retrieves instance pointer for a particular index
func (rs *RoutingState) GetShard(shardID string) (shardInst *shard.Instance) {
	instancePairs := rs.manager.routing.Load().pairs
	pair, ok := instancePairs[shardID]
	if !ok {
		return
	}
	shardInst = pair.Shard
	return
}

// Checks if particular instance has been marked as shutdown
func (rs *RoutingState) IsShardShutdown(shardID string) (shutdown bool) {
	instancePairs := rs.manager.routing.Load().pairs
	pair, ok := instancePairs[shardID]
	if !ok {
		shutdown = true
		return
	}
	shardInst := pair.Shard
	shutdown = shardInst.InShutdown.Load()
	return
}
