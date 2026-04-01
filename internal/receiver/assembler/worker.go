package assembler

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
	"time"
)

func (manager *Manager) newWorker(shard *shard.Instance) (new *Instance) {
	new = &Instance{
		Shard:   shard,
		outbox:  manager.outQueue,
		Metrics: MetricStorage{},
	}
	return
}

func (instance *Instance) run() {
	ctx := instance.ctx

	for {
		// Stop this worker when cancel requested
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			// Record panics and continue defrag
			defer func() {
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogStdErr(ctx, "panic in assembler worker thread: %v\n%s", fatalError, stack)
				}
			}()

			bucketKey, ok := instance.Shard.PopKey(ctx)
			if !ok {
				if ctx.Err() == context.Canceled {
					// Normal exit procedure
					return
				}

				logctx.LogStdWarn(ctx, "failed to retrieve waiting bucket from shard-to-assembler queue\n")
				return
			}

			start := time.Now()
			bucket, notExist := instance.Shard.DrainBucket(ctx, bucketKey)
			if notExist {
				return
			}

			var fragSlice []protocol.Payload
			for _, fragment := range bucket.Fragments {
				fragSlice = append(fragSlice, fragment)
			}

			finalMsg, err := protocol.Defragment(fragSlice)
			if err != nil {
				logctx.LogStdErr(ctx, "Failed assembler: %w\n", err)
				return
			}

			// Record time metrics post-validation
			durNs := time.Since(start).Nanoseconds()
			instance.Metrics.SumNs.Add(uint64(durNs))
			oldMax := int64(instance.Metrics.MaxNs.Load())
			for {
				if durNs > oldMax {
					if instance.Metrics.MaxNs.CompareAndSwap(uint64(oldMax), uint64(durNs)) {
						break
					}
					oldMax = int64(instance.Metrics.MaxNs.Load())
				} else {
					break
				}
			}

			// Push combined message to Stage 4 queue
			maxRetries := 10
			retryWait := 10 * time.Millisecond
			for range maxRetries {
				err = instance.outbox.Push(finalMsg, uint64(finalMsg.Size()))
				if err == nil {
					break
				}

				time.Sleep(retryWait)
			}
			if err != nil {
				logctx.LogStdErr(ctx,
					"Failed to push message to output queue: host id %d, message id %d, hostname %s: %w\n",
					finalMsg.HostID, finalMsg.MsgID, finalMsg.Hostname, err)
				return
			}

			instance.Metrics.ProcessedBuckets.Add(1) // increment success after push

			logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog, "Processed bucket %v\n", bucketKey)
		}()
	}
}
