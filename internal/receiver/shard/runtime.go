package shard

import (
	"context"
	"maps"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"time"
)

// Shard deadline watcher - ensures buckets that exceed deadline are marked filled
func (queue *Instance) StartTimeoutWatcher(ctx context.Context) {
	ctx = logctx.AppendCtxTag(ctx, logctx.NSWatcher)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	deadlinePtr := queue.packetDeadline

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
					logctx.LogStdErr(ctx,
						"panic in shard watcher thread: %v\n%s", fatalError, stack)
				}
			}()

			// Load current deadline value
			packetDeadline := time.Duration(deadlinePtr.Load())

			// Periodically check buckets in each shard to see if they have timed out
			time.Sleep(200 * time.Millisecond)

			// Gather list of current buckets
			queue.Mu.Lock()
			refs := make(map[string]*Bucket, len(queue.Buckets))
			maps.Copy(refs, queue.Buckets)
			queue.Mu.Unlock()

			// Check all buckets in each shard for timeout
			for bucketKey, bucket := range refs {
				bucket.Mutex.Lock()
				if bucket.filled {
					bucket.Mutex.Unlock()
					continue
				}

				if time.Since(bucket.lastProcessStartTime) > packetDeadline {
					// If the bucket has timed out, process it
					bucket.filled = true
					bucket.Mutex.Unlock()

					queue.Metrics.TimedOutBuckets.Add(1)
					queue.Metrics.WaitingBuckets.Add(1)
					select {
					case queue.keyQueue <- bucketKey:
					case <-ctx.Done():
						if !atomics.Subtract(&queue.Metrics.WaitingBuckets, 1) {
							logctx.LogStdWarn(ctx, "Context cancelled, failed to decrement false waiting bucket metric for bucket %s\n",
								bucketKey)
						}
						return
					}

					bucket.Mutex.RLock()
					var haveSeq []int
					for _, fragment := range bucket.Fragments {
						haveSeq = append(haveSeq, fragment.MessageSeq)
					}
					maxSeq := bucket.maxSeq
					bucket.Mutex.RUnlock()

					if len(haveSeq) < 100 {
						logctx.LogStdWarn(ctx, "Bucket %s timed out (expected %d packets within %s, only received %d sequences %v)\n",
							bucketKey, maxSeq+1, packetDeadline.String(), len(haveSeq), haveSeq)
					} else {
						logctx.LogStdWarn(ctx, "Bucket %s timed out (expected %d packets within %s, only received %d sequences)\n",
							bucketKey, maxSeq+1, packetDeadline.String(), len(haveSeq))
					}
				} else {
					bucket.Mutex.Unlock()
				}
			}
		}()
	}
}
