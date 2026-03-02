package processor

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
	"time"
)

// Creates new processor with requested queue as inbox
func (manager *Manager) newWorker() (new *Instance) {
	new = &Instance{
		namespace:            append(logctx.GetTagList(manager.ctx), logctx.NSWorker),
		pastTimestampLimit:   manager.Config.PastMsgCutoff,
		futureTimestampLimit: manager.Config.FutureMsgCutoff,
		inbox:                manager.Inbox,
		routingView:          manager.routingView,
		Metrics:              MetricStorage{},
	}
	return
}

func (instance *Instance) run(ctx context.Context) {
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
					logctx.LogStdErr(ctx,
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
				logctx.LogStdErr(ctx, "failed outer payload deconstruction: %s\n", err.Error())
				return
			}

			payload, err := protocol.DeconstructInnerPayload(innerPayload)
			if err != nil {
				logctx.LogStdErr(ctx, "failed inner payload deconstruction: %s\n", err.Error())
				return
			}

			msg, err := protocol.ParsePayload(payload)
			if err != nil {
				logctx.LogStdErr(ctx, "invalid payload: %s\n", err.Error())
				return
			}

			// Inject remote IP to actual message
			msg.RemoteIP = queueEntry.Meta.RemoteIP

			// Validate timestamp - disallow extremes always
			if msg.Timestamp.After(processingStartTime.Add(instance.futureTimestampLimit)) {
				logctx.LogStdErr(ctx,
					"message from %s (msgID: %s) has a timestamp too far in the future (>=%d hours), dropping\n",
					msg.RemoteIP, msg.MsgID, instance.futureTimestampLimit.Hours())
				return
			} else if msg.Timestamp.Before(processingStartTime.Add(-instance.pastTimestampLimit)) {
				logctx.LogStdErr(ctx,
					"message from %s (msgID: %s) has a timestamp too far in the past (<%d hours), dropping\n",
					msg.RemoteIP, msg.MsgID, instance.pastTimestampLimit.Hours())
				return
			}

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
