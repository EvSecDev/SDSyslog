// Fragments log messages into pieces fitting within maximum payload size for network
package assembler

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/listener"
	"sdsyslog/pkg/protocol"
)

func New(namespace []string, inQueue *mpmc.Queue[listener.ParsedMessage], outQueue *mpmc.Queue[[]byte], hostID, maxPayloadSize int) (new *Instance) {
	new = &Instance{
		Namespace:      append(namespace, global.NSAssm),
		inbox:          inQueue,
		outbox:         outQueue,
		hostID:         hostID,
		maxPayloadSize: maxPayloadSize,
		Metrics:        &MetricStorage{},
	}
	return
}

func (instance *Instance) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			// Record panics and continue working
			defer func() {
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"panic in assembler worker thread: %v\n%s", fatalError, stack)
				}
			}()

			container, ok := instance.inbox.Pop(ctx)
			if !ok {
				return
			}
			size := len(container.Text) +
				len(container.ApplicationName) +
				len(container.Hostname) +
				len(container.Facility) +
				len(container.Severity) +
				16 // int64 size pid and time
			// Subtract data size from sum
			atomics.Subtract(&instance.inbox.ActiveWrite.Load().Metrics.Bytes, uint64(size), 4)

			newMsg := protocol.Message{
				Facility:        container.Facility,
				Severity:        container.Severity,
				Timestamp:       container.Timestamp,
				ProcessID:       container.ProcessID,
				Hostname:        container.Hostname,
				ApplicationName: container.ApplicationName,
				LogText:         container.Text,
			}

			msgLengthB := uint64(len(newMsg.LogText))

			instance.Metrics.TotalMessages.Add(1)
			instance.Metrics.TotalMsgSizeBytes.Add(msgLengthB)

			maxSeenMsgSizeBytes := instance.Metrics.MaxMsgSizeBytes.Load()
			if msgLengthB > maxSeenMsgSizeBytes {
				instance.Metrics.MaxMsgSizeBytes.CompareAndSwap(maxSeenMsgSizeBytes, msgLengthB)
			}

			packets, err := protocol.Create(newMsg, instance.hostID, instance.maxPayloadSize)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed serialization: %v", err)
				return
			}

			fragmentCount := uint64(len(packets))

			instance.Metrics.TotalFragmentCtn.Add(fragmentCount)

			maxSeenFragmentCount := instance.Metrics.MaxFragmentCtn.Load()
			if fragmentCount > maxSeenFragmentCount {
				instance.Metrics.MaxFragmentCtn.CompareAndSwap(maxSeenFragmentCount, fragmentCount)
			}

			for _, packet := range packets {
				success := instance.outbox.Push(packet)
				if !success {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"Sender queue full, fragment dropped\n")
					continue
				}
				// Add data size to sum
				instance.outbox.ActiveWrite.Load().Metrics.Bytes.Add(uint64(len(packet)))
			}
		}()
	}
}
