package assembler

import (
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
)

func (manager *Manager) newWorker() (new *Instance) {
	if manager == nil {
		return
	}

	new = &Instance{
		inbox:          manager.InQueue,
		outbox:         manager.outQueue,
		Metrics:        MetricStorage{},
		hostID:         manager.Config.HostID,
		maxPayloadSize: manager.Config.MaxPayloadSize,
		cryptoSuiteID:  manager.Config.CryptoSuiteID,
		sigSuiteID:     manager.Config.SigSuiteID,
	}
	return
}

func (instance *Instance) run() {
	if instance == nil {
		return
	}

	ctx := instance.ctx

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
					logctx.LogStdErr(ctx,
						"panic in assembler worker thread: %v\n%s", fatalError, stack)
				}
			}()

			container, ok := instance.inbox.Pop(ctx)
			if !ok {
				return
			}
			// Subtract data size from sum
			atomics.Subtract(&instance.inbox.ActiveWrite.Load().Metrics.Bytes, uint64(container.Size()), 4)

			// In-module added fields
			customFields := make(map[string]any)
			for key, val := range container.Fields {
				customFields[key] = val
			}

			newMsg := protocol.Message{
				Timestamp: container.Timestamp,
				Hostname:  container.Hostname,
				Fields:    customFields,
				Data:      container.Data,
			}

			msgLengthB := uint64(len(newMsg.Data))

			instance.Metrics.TotalMessages.Add(1)
			instance.Metrics.TotalMsgSizeBytes.Add(msgLengthB)

			maxSeenMsgSizeBytes := instance.Metrics.MaxMsgSizeBytes.Load()
			if msgLengthB > maxSeenMsgSizeBytes {
				instance.Metrics.MaxMsgSizeBytes.CompareAndSwap(maxSeenMsgSizeBytes, msgLengthB)
			}

			packets, err := protocol.Create(newMsg,
				instance.hostID,
				instance.maxPayloadSize,
				instance.cryptoSuiteID,
				instance.sigSuiteID)
			if err != nil {
				logctx.LogStdErr(ctx, "failed serialization: %w\n", err)
				return
			}

			fragmentCount := uint64(len(packets))

			instance.Metrics.TotalFragmentCtn.Add(fragmentCount)

			maxSeenFragmentCount := instance.Metrics.MaxFragmentCtn.Load()
			if fragmentCount > maxSeenFragmentCount {
				instance.Metrics.MaxFragmentCtn.CompareAndSwap(maxSeenFragmentCount, fragmentCount)
			}

			for _, packet := range packets {
				success := instance.outbox.Push(packet, uint64(len(packet)))
				if !success {
					logctx.LogStdErr(ctx,
						"Sender queue full, fragment dropped\n")
					continue
				}
			}
		}()
	}
}
