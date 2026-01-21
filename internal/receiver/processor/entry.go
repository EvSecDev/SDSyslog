// Deserializes and decrypts received packets into message fragments
package processor

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
	"time"
)

// Creates new processor with requested queue as inbox
func New(namespace []string, queue *mpmc.Queue[listener.Container], shardRouting shard.RoutingView) (new *Instance) {
	new = &Instance{
		Namespace:   append(namespace, global.NSWorker),
		inbox:       queue,
		routingView: shardRouting,
		Metrics:     MetricStorage{},
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
			// Record panics and continue processing
			defer func() {
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"panic in processor worker thread: %v\n%s", fatalError, stack)
				}
			}()

			queueEntry, received := instance.inbox.Pop(ctx)
			if !received {
				return
			}
			size := len(queueEntry.Data) + len(queueEntry.Meta.RemoteIP)
			// Subtract data size from sum
			atomics.Subtract(&instance.inbox.ActiveWrite.Load().Metrics.Bytes, uint64(size), 4)

			processingStartTime := time.Now() // Record start time immediately after we read the queue entry

			data := queueEntry.Data

			innerPayload, err := protocol.DeconstructOuterPayload(data)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "%s\n", err.Error())
				return
			}

			payload, err := protocol.DeconstructInnerPayload(innerPayload)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "%s\n", err.Error())
				return
			}

			msg, err := protocol.ParsePayload(payload)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "%s\n", err.Error())
				return
			}

			// Inject remote IP to actual message
			msg.RemoteIP = queueEntry.Meta.RemoteIP

			// Record time metrics post-validation
			durNs := time.Since(processingStartTime).Nanoseconds()
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

			success := shard.RouteFragment(ctx, instance.routingView, queueEntry.Meta.RemoteIP, msg, processingStartTime)
			if !success {
				return
			}
			instance.Metrics.ValidPayloads.Add(1)
		}()
	}
}
