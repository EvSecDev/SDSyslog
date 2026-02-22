package defrag

import (
	"sdsyslog/internal/receiver/shard"
)

// Current number of shards running
func (rs *RoutingState) GetShardCount() (count int) {
	rs.Manager.Mu.RLock()
	defer rs.Manager.Mu.RUnlock()
	count = len(rs.Manager.InstancePairs)
	return
}

// Retrieves instance pointer for a particular index
func (rs *RoutingState) GetShard(shardID int) (shardInst *shard.Instance) {
	rs.Manager.Mu.RLock()
	defer rs.Manager.Mu.RUnlock()
	if len(rs.Manager.InstancePairs)-1 >= shardID {
		shardInst = rs.Manager.InstancePairs[shardID].Shard
	}
	return
}

// Checks if particular instance has been marked as shutdown
func (rs *RoutingState) IsShardShutdown(shardID int) (shutdown bool) {
	rs.Manager.Mu.RLock()
	defer rs.Manager.Mu.RUnlock()
	if len(rs.Manager.InstancePairs)-1 >= shardID {
		shutdown = rs.Manager.InstancePairs[shardID].Shard.InShutdown
	}
	return
}

// Checks if shard contains a particular bucket
func (rs *RoutingState) BucketExists(shardID int, bucketKey string) (present bool) {
	rs.Manager.Mu.RLock()
	sh := rs.Manager.InstancePairs[shardID].Shard
	rs.Manager.Mu.RUnlock()

	sh.Mu.Lock()
	defer sh.Mu.Unlock()
	_, present = sh.Buckets[bucketKey]
	return
}

// Checks if any shard contains a particular bucket
func (rs *RoutingState) BucketExistsAnywhere(bucketKey string) (bucketExists bool) {
	rs.Manager.Mu.RLock()
	defer rs.Manager.Mu.RUnlock()
	for _, pair := range rs.Manager.InstancePairs {
		pair.Shard.Mu.Lock()
		_, bucketExists = pair.Shard.Buckets[bucketKey]
		pair.Shard.Mu.Unlock()

		if bucketExists {
			return
		}
	}

	return
}

// Retrieves bucket overridden shard destination, if any
func (rs *RoutingState) GetOverride(bucketKey string) (overrideShard int, hasOverride bool) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	overrideShard, hasOverride = rs.Overrides[bucketKey]
	return
}

// Adds an override for a bucket. Routes all further buckets to desired shard id
func (rs *RoutingState) SetOverride(bucketKey string, shardID int) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	rs.Overrides[bucketKey] = shardID
}

// Removes bucket routing override
func (rs *RoutingState) ClearOverride(bucketKey string) {
	rs.Mu.Lock()
	defer rs.Mu.Unlock()
	delete(rs.Overrides, bucketKey)
}

// Picks the first non-shutdown shard that is not the original one
func (rs *RoutingState) FindAlternativeShard(origShardID int) (newShardID int) {
	rs.Manager.Mu.RLock()
	defer rs.Manager.Mu.RUnlock()
	for shardID, pair := range rs.Manager.InstancePairs {
		if shardID != origShardID && !pair.Shard.InShutdown {
			newShardID = shardID
			return
		}
	}

	// Should never hit this
	newShardID = origShardID
	return
}

// Identifies if there is any single shard in a non-draining state (accepting new and existing fragments).
// Running is true when at least one shard is accepting new and existing.
// If no shards could be found, returns false for running and draining.
func (rs *RoutingState) ShardsAvailable() (running bool, draining bool) {
	rs.Manager.Mu.RLock()
	defer rs.Manager.Mu.RUnlock()

	if len(rs.Manager.InstancePairs) == 0 {
		return
	}

	for _, pair := range rs.Manager.InstancePairs {
		pair.Shard.Mu.Lock()
		shardDraining := pair.Shard.InShutdown
		pair.Shard.Mu.Unlock()

		// Found a shard accepting new fragments
		if !shardDraining {
			running = true
			return
		}
	}

	return
}
