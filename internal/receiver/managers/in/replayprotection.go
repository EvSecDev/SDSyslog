package in

import (
	"context"
	"hash/fnv"
	"time"
)

// Create new replay cache with n shards and TTL
func newReplayCacheWithShards(numShards int, ttlSeconds int64) (newCache *replayCache) {
	newCache = &replayCache{
		shards: make([]*replayCacheShard, numShards),
		ttl:    ttlSeconds,
	}
	for i := 0; i < numShards; i++ {
		newCache.shards[i] = &replayCacheShard{
			store: make(map[string]int64, 4096),
		}
	}
	return
}

// Pick shard deterministically based on key
func (cache *replayCache) getShard(publicKey []byte) (shard *replayCacheShard) {
	h := fnv.New32a()
	h.Write(publicKey)
	id := h.Sum32()

	shardNum := uint32(len(cache.shards))

	shard = cache.shards[id%shardNum]
	return
}

// Check if public key has been seen within the replay attack protection window
func (cache *replayCache) isReplayed(pubKey []byte) (replay bool) {
	now := time.Now().Unix()
	key := string(pubKey) // string conversion to use as map key
	shard := cache.getShard(pubKey)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	seenTime, seen := shard.store[key]
	if seen && now-seenTime < cache.ttl {
		replay = true
		return
	}

	shard.store[key] = now
	return
}

// Cache eviction worker - ensures seen keys are cleaned up according to TTL
func (cache *replayCache) cleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, shard := range cache.shards {
				cache.cleanupShard(shard)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Cleanup a single shard
func (cache *replayCache) cleanupShard(shard *replayCacheShard) {
	now := time.Now().Unix()
	shard.mu.Lock()
	defer shard.mu.Unlock()
	for key, seenTime := range shard.store {
		if now-seenTime >= cache.ttl {
			delete(shard.store, key)
		}
	}
}
