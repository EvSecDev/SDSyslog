// Reads packets from the network and conducts pre-validation
package listener

import (
	"context"
	"errors"
	"net"
	"runtime/debug"
	"sdsyslog/internal/crypto"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"time"
)

func New(namespace []string, conn *net.UDPConn, queue *mpmc.Queue[Container]) (new *Instance) {
	new = &Instance{
		Namespace: append(namespace, global.NSListen),
		conn:      conn,
		Outbox:    queue,
		minLen:    protocol.MinOuterPayloadLen,
		Metrics:   MetricStorage{},
	}
	return
}

func (instance *Instance) Run(ctx context.Context) {
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
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "panic in listener worker thread: %v\n%s", fatalError, stack)
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
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Failed reading data from socket: %w\n", err)
				instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
				return
			}

			payload := append([]byte(nil), buffer[:endIndex]...)

			// Pre validation
			_, validSuiteID := crypto.GetSuiteInfo(payload[0])
			if len(payload) < instance.minLen || !validSuiteID {
				instance.Metrics.InvalidPackets.Add(1)
				instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
				logctx.LogEvent(ctx, global.VerbosityProgress, global.WarnLog, "Received invalid outer payload from %s (crypto id %d)\n", remoteAddr.String(), payload[0])
				return
			}

			var newQueueEntry Container
			newQueueEntry.Data = payload
			newQueueEntry.Meta.RemoteIP = remoteAddr.IP.String()

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

			instance.Outbox.Push(newQueueEntry)
			// Add data size to sum
			size := len(newQueueEntry.Data) + len(newQueueEntry.Meta.RemoteIP)
			instance.Outbox.ActiveWrite.Load().Metrics.Bytes.Add(uint64(size))
			instance.Metrics.ValidPackets.Add(1) // increment success after push
			instance.Metrics.BusyNs.Add(uint64(time.Since(start)))
		}()
	}
}
