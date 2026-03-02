package output

import (
	"context"
	"net"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
)

func newWorker(namespace []string, inQueue *mpmc.Queue[[]byte], conn *net.UDPConn) (new *Instance) {
	new = &Instance{
		namespace: append(namespace, logctx.NSWorker),
		inbox:     inQueue,
		conn:      conn,
		Metrics:   MetricStorage{},
	}
	return
}

func (instance *Instance) run(ctx context.Context) {
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
						"panic in output worker thread: %v\n%s", fatalError, stack)
				}
			}()

			frag, ok := instance.inbox.Pop(ctx)
			if !ok {
				return
			}
			// Subtract data size from sum
			atomics.Subtract(&instance.inbox.ActiveWrite.Load().Metrics.Bytes, uint64(len(frag)), 4)

			_, err := instance.conn.Write(frag)
			if err != nil {
				logctx.LogStdErr(ctx,
					"Failed to send fragment: %w\n", err)
				return
			}

			pktLengthB := uint64(len(frag))
			instance.Metrics.SumPacketBytes.Add(pktLengthB)

			maxSeenPktBytes := instance.Metrics.MaxPacketBytes.Load()
			if pktLengthB > maxSeenPktBytes {
				instance.Metrics.MaxPacketBytes.CompareAndSwap(maxSeenPktBytes, pktLengthB)
			}

			instance.Metrics.TotalPackets.Add(1)

			logctx.LogEvent(ctx, logctx.VerbosityData, logctx.InfoLog,
				"Sent fragment (size %d) to %s\n", len(frag), instance.conn.RemoteAddr())
		}()
	}
}
