package shard

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"time"
)

// Shard deadline watcher - ensures buckets that exceed deadline are marked filled
func (shard *Instance) StartTimeoutWatcher(ctx context.Context) {
	ctx = logctx.AppendCtxTag(ctx, global.NSWatcher)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	deadlinePtr := shard.PacketDeadline

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			// Record panics and continue watching
			defer func() {
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"panic in shard watcher thread: %v\n%s", fatalError, stack)
				}
			}()

			// Load current deadline value
			packetDeadline := time.Duration(deadlinePtr.Load())

			// Periodically check buckets in each shard to see if they have timed out
			time.Sleep(200 * time.Millisecond)

			// Check all buckets in each shard for timeout
			keysToSend := []string{}
			shard.Mu.Lock()

			for bucketKey, bucket := range shard.Buckets {
				if bucket.filled {
					continue
				}

				if time.Since(bucket.lastProcessStartTime) > packetDeadline {
					// If the bucket has timed out, process it
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Bucket %s timed out\n", bucketKey)

					// Process the bucket
					bucket.filled = true
					keysToSend = append(keysToSend, bucketKey)
					shard.Metrics.TimedOutBuckets.Add(1)
				}
			}
			shard.Mu.Unlock()

			for _, bucketKey := range keysToSend {
				select {
				case shard.KeyQueue <- bucketKey:
					shard.Metrics.WaitingBuckets.Add(1)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}
