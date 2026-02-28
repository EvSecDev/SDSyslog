// Handles buffering message fragments, timing out partial messages, and load balancing fragments to assemblers
package shard

import (
	"context"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"sync/atomic"
	"time"
)

// Create new shard
func New(namespace []string, buffer int, packetDeadlinePtr *atomic.Int64) (new *Instance) {
	new = &Instance{
		Namespace:      append(namespace, logctx.NSQueue),
		Buckets:        make(map[string]*Bucket),
		KeyQueue:       make(chan string, buffer),
		PacketDeadline: packetDeadlinePtr,
		Metrics:        MetricStorage{},
	}
	return
}

// Add fragment to bucket
func (queue *Instance) push(ctx context.Context, bucketKey string, fragment protocol.Payload, processingStartTime time.Time) {
	queue.Mu.Lock()
	defer queue.Mu.Unlock()

	queue.Metrics.PushCount.Add(1)

	bucket, ok := queue.Buckets[bucketKey]
	if !ok {
		bucket = &Bucket{
			Fragments: make(map[int]protocol.Payload),
			maxSeq:    fragment.MessageSeqMax,
		}
		queue.Buckets[bucketKey] = bucket
		queue.Metrics.TotalBuckets.Add(1)
	} else {
		// Discard newest fragment if duplicate keys exist within the deadline
		if bucket.filled {
			return
		}

		// Discard newest fragment if message sequence doesn't match last
		// Root of trust is first fragment received
		if bucket.maxSeq != fragment.MessageSeqMax {
			return
		}
	}

	// Record time spacing between fragments
	elapsed := time.Since(bucket.lastProcessStartTime)
	if elapsed > 0 {
		queue.Metrics.SumFragmentTimeSpacing.Add(uint64(elapsed))
	}

	// Update process time always, acts as modified time
	bucket.lastProcessStartTime = processingStartTime

	// Even though this should never occur, evaluate deadline anyways in case a remote end tries to sneak a false packet in
	if time.Since(processingStartTime) > time.Duration(queue.PacketDeadline.Load()) {
		bucket.filled = true

		select {
		case <-ctx.Done():
			return
		case queue.KeyQueue <- bucketKey:
			// success
			queue.Metrics.WaitingBuckets.Add(1)
			queue.Metrics.TimedOutBuckets.Add(1)
			return
		}
	}

	// Store fragment by sequence number
	bucket.Fragments[fragment.MessageSeq] = fragment
	queue.Metrics.Bytes.Add(uint64(fragment.Size()))

	// Check if bucket is now filled
	if len(bucket.Fragments) == bucket.maxSeq+1 {
		bucket.filled = true

		select {
		case <-ctx.Done():
			return
		case queue.KeyQueue <- bucketKey:
			// success
			queue.Metrics.WaitingBuckets.Add(1)
			return
		}
	}
}

// Retrieve one bucket key from shard's queue
func (queue *Instance) PopKey(ctx context.Context) (key string, ok bool) {
	queue.Metrics.PopCount.Add(1)

	select {
	case <-ctx.Done():
		return
	case key, ok = <-queue.KeyQueue:
		if ok {
			// Protecting subtract - will get out of sync without explicit sync between assembler and processors
			queue.Mu.Lock()
			success := atomics.Subtract(&queue.Metrics.WaitingBuckets, 1, 1) // max retries set low due to single consumer (assembler itself)
			queue.Mu.Unlock()
			if !success {
				logctx.LogStdWarn(ctx,
					"failed to decrement waiting bucket metric after successful bucket key retrieval\n")
			}
			return
		}
	}
	return
}

// Retrieve (remove) bucket from shard's storage
func (queue *Instance) DrainBucket(ctx context.Context, key string) (bucket *Bucket, bucketNotExist bool) {
	queue.Mu.Lock()
	defer queue.Mu.Unlock()

	// Retrieve bucket
	bucket, ok := queue.Buckets[key]
	if !ok {
		queue.Mu.Unlock()
		bucketNotExist = true
		return
	}

	// Remove bucket from storage
	delete(queue.Buckets, key)

	// Subtract data size from sum
	var size int
	for _, frag := range bucket.Fragments {
		size += frag.Size()
	}
	atomics.Subtract(&queue.Metrics.Bytes, uint64(size), 1)

	// Decrement bucket count
	success := atomics.Subtract(&queue.Metrics.TotalBuckets, 1, 1) // max retries set low due to single consumer (assembler itself)
	if !success {
		logctx.LogStdWarn(ctx,
			"failed to decrement total bucket metric after successful bucket deletion\n")
	}
	return
}
