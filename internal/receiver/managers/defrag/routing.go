package defrag

import (
	"sdsyslog/internal/receiver/shard"
)

// Current number of shards running
func (rs *RoutingState) GetShardCount() (count int) {
	count = len(rs.Manager.InstancePairs)
	return
}

// Retrieves instance pointer for a particular index
func (rs *RoutingState) GetShard(shardID int) (shardInst *shard.Instance) {
	if len(rs.Manager.InstancePairs)-1 >= shardID {
		shardInst = rs.Manager.InstancePairs[shardID].Shard
	}
	return
}

// Checks if particular instance has been marked as shutdown
func (rs *RoutingState) IsShardShutdown(shardID int) (shutdown bool) {
	if len(rs.Manager.InstancePairs)-1 >= shardID {
		shutdown = rs.Manager.InstancePairs[shardID].Shard.InShutdown
	}
	return
}

// Checks if shard contains a particular bucket
func (rs *RoutingState) BucketExists(shardID int, bucketKey string) (present bool) {
	sh := rs.Manager.InstancePairs[shardID].Shard
	sh.Mu.Lock()
	_, present = sh.Buckets[bucketKey]
	sh.Mu.Unlock()
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
