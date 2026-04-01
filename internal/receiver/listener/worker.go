package listener

import (
	"errors"
	"net"
	"runtime/debug"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/crypto/registry"
	"sdsyslog/pkg/protocol"
	"time"
)

func (manager *Manager) newWorker(conn *net.UDPConn) (new *Instance) {
	if manager == nil {
		return
	}

	new = &Instance{
		conn:       conn,
		outbox:     manager.outbox,
		minLen:     protocol.MinOuterPayloadLen,
		Metrics:    MetricStorage{},
		isReplayed: manager.replayCache.isReplayed,
	}
	return
}

func (instance *Instance) run() {
	if instance == nil {
		return
	}

	ctx := instance.ctx
	buffer := make([]byte, 65535)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		func() {
			defer func() {
				// Record panics and continue listening
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogStdErr(ctx, "panic in listener worker thread: %v\n%s", fatalError, stack)
				}
			}()

			// Blocking until data or connection is closed by manager
			endIndex, remoteAddr, err := instance.conn.ReadFromUDP(buffer)
			start := time.Now() // Record start time immediately after we read the packet
			if err != nil {
				if ctx.Err() != nil {
					// Cancellation received, graceful shutdown
					return
				}

				// If conn closed but ctx NOT canceled, return - treat as shutdown
				if errors.Is(err, net.ErrClosed) {
					return
				}

				// Otherwise, regular error
				logctx.LogStdErr(ctx, "Failed reading data from socket: %w\n", err)
				instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
				return
			}

			if endIndex == 0 {
				// Empty packets are always ignored
				return
			}

			payload := append([]byte(nil), buffer[:endIndex]...)

			// Pre validation
			suiteInfo, validSuiteID := registry.GetSuiteInfo(payload[0])
			if len(payload) < instance.minLen || !validSuiteID {
				instance.Metrics.InvalidPackets.Add(1)
				instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
				logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.WarnLog,
					"Received invalid outer payload from %s (crypto id %d)\n", remoteAddr.String(), payload[0])
				return
			}

			// Replay attack protection - level 1
			//   minLen protects against packets smaller than key sizes
			pubKey := payload[registry.SuiteIDLen : registry.SuiteIDLen+suiteInfo.KeySize]
			if instance.isReplayed(pubKey) {
				instance.Metrics.InvalidPackets.Add(1)
				instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
				logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.WarnLog,
					"Received replayed outer payload from %s (crypto id %d)\n", remoteAddr.String(), payload[0])
				return
			}

			var newQueueEntry Container
			newQueueEntry.Data = payload
			newQueueEntry.Meta.RemoteIP = remoteAddr.AddrPort().Addr()

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

			const netipAddrSize = 24
			size := len(newQueueEntry.Data) + netipAddrSize

			err = instance.outbox.PushWithRetry(newQueueEntry, uint64(size), 4)
			if err != nil {
				logctx.LogStdWarn(ctx, "failed to push packet from %q to processor queue: %w\n",
					remoteAddr.String(), err)
				return
			}
			instance.Metrics.ValidPackets.Add(1) // increment pkt count after push (success or not)
			instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
		}()
	}
}
