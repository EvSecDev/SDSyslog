// Reassembles message fragments into original message order
package assembler

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
	"time"
)

func New(namespace []string, shard *shard.Instance, outQueue *mpmc.Queue[protocol.Payload], overrideClear shard.OverrideCleaner) (new *Instance) {
	new = &Instance{
		Namespace: append(namespace, global.NSAssm),
		shardInst: shard,
		outbox:    outQueue,
		cleaner:   overrideClear,
		Metrics:   &MetricStorage{},
	}
	return
}

func (instance *Instance) Run(ctx context.Context) {
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
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"panic in assembler worker thread: %v\n%s", fatalError, stack)
				}
			}()

			bucketKey, ok := instance.shardInst.PopKey(ctx)
			if !ok {
				if ctx.Err() == context.Canceled {
					// Normal exit procedure
					return
				}

				logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
					"failed to retrieve waiting bucket from shard-to-assembler queue\n")
				return
			}

			start := time.Now()
			bucket, notExist := instance.shardInst.DrainBucket(ctx, bucketKey)
			if notExist {
				return
			}

			instance.cleaner.ClearOverride(bucketKey)

			var fragSlice []protocol.Payload
			for _, fragment := range bucket.Fragments {
				fragSlice = append(fragSlice, fragment)
			}

			finalMsg, err := protocol.Defragment(fragSlice)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"Failed assembler: %v\n", err)
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
			maxRetries := 4
			retryWait := 5 * time.Millisecond
			success := false
			for range maxRetries {
				success = instance.outbox.Push(finalMsg)
				if success {
					// Add data size to sum
					size := finalMsg.Size()
					instance.outbox.ActiveWrite.Load().Metrics.Bytes.Add(uint64(size))
					break
				}

				time.Sleep(retryWait)
			}
			if !success {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"Failed to push message to output queue: host id %d, log id %d, hostname %s, appname %s\n",
					finalMsg.HostID, finalMsg.LogID, finalMsg.Hostname, finalMsg.ApplicationName)
				return
			}

			instance.Metrics.ProcessedBuckets.Add(1) // increment success after push

			logctx.LogEvent(ctx, global.VerbosityData, global.InfoLog, "Processed bucket %v\n", bucketKey)
		}()
	}
}
