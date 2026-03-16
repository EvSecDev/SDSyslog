package processor

import (
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
	"time"
)

// Creates new processor with requested queue as inbox
func (manager *Manager) newWorker() (new *Instance) {
	if manager == nil {
		return
	}

	new = &Instance{
		pastTimestampLimit:   manager.Config.PastMsgCutoff,
		futureTimestampLimit: manager.Config.FutureMsgCutoff,
		inbox:                manager.Inbox,
		routingView:          manager.routingView,
		Metrics:              MetricStorage{},
	}
	return
}

func (instance *Instance) run() {
	if instance == nil {
		return
	}

	ctx := instance.ctx

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
			processingStartTime := time.Now() // Record start time immediately after we read the queue entry

			size := len(queueEntry.Data) + 24 // netip.Addr obj size
			// Subtract data size from sum
			atomics.Subtract(&instance.inbox.ActiveWrite.Load().Metrics.Bytes, uint64(size), 4)

			defer func() {
				// Record busy time when worker is done processing this packet (valid or not)
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
			}()

			data := queueEntry.Data

			innerPayload, err := protocol.DeconstructOuterPayload(data)
			if err != nil {
				logctx.LogStdErr(ctx, "failed outer payload deconstruction (source: %s): %s\n",
					queueEntry.Meta.RemoteIP.String(), err.Error())
				instance.Metrics.InvalidPayloads.Add(1)
				return
			}

			payload, err := protocol.DeconstructInnerPayload(innerPayload)
			if err != nil {
				logctx.LogStdErr(ctx, "failed inner payload deconstruction (source: %s): %s\n",
					queueEntry.Meta.RemoteIP.String(), err.Error())
				instance.Metrics.InvalidPayloads.Add(1)
				return
			}

			msg, err := protocol.DeconstructPayload(payload)
			if err != nil {
				logctx.LogStdErr(ctx, "invalid payload (source: %s): %s\n",
					queueEntry.Meta.RemoteIP.String(), err.Error())
				instance.Metrics.InvalidPayloads.Add(1)
				return
			}

			// Inject remote IP to actual message
			msg.RemoteIP = queueEntry.Meta.RemoteIP

			// Validate timestamp - disallow extremes always
			if msg.Timestamp.After(processingStartTime.Add(instance.futureTimestampLimit)) {
				logctx.LogStdErr(ctx,
					"message from %s (msgID: %d) has a timestamp too far in the future (>=%s), dropping\n",
					msg.RemoteIP.String(), msg.MsgID, instance.futureTimestampLimit.String())
				instance.Metrics.InvalidPayloads.Add(1)
				return
			} else if msg.Timestamp.Before(processingStartTime.Add(-instance.pastTimestampLimit)) {
				logctx.LogStdErr(ctx,
					"message from %s (msgID: %d) has a timestamp too far in the past (<%s), dropping\n",
					msg.RemoteIP.String(), msg.MsgID, instance.pastTimestampLimit.String())
				instance.Metrics.InvalidPayloads.Add(1)
				return
			}

			instance.Metrics.ValidPayloads.Add(1)

			shard.RouteFragment(ctx, instance.routingView, queueEntry.Meta.RemoteIP, msg, processingStartTime)
		}()
	}
}
