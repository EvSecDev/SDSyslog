// Handles writing final assembled log messages to configured output destinations (file, journald, ect.)
package output

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"time"
)

// Creates new worker instance
func New(namespace []string, inQueue *mpmc.Queue[protocol.Payload]) (new *Instance) {
	new = &Instance{
		Namespace: append(namespace, global.NSWorker),
		Inbox:     inQueue,
		Metrics:   MetricStorage{},
	}
	return
}

// Take assembled messages and write to configured outputs
func (instance *Instance) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	popCh := make(chan protocol.Payload, 1)

	go func() {
		for {
			msg, ok := instance.Inbox.Pop(ctx)
			if !ok {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			popCh <- msg
			// Subtract data size from sum
			size := msg.Size()
			atomics.Subtract(&instance.Inbox.ActiveWrite.Load().Metrics.Bytes, uint64(size), 4)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			if instance.FileMod != nil {
				instance.FileMod.FlushBuffer()
			}
			return
		case <-ticker.C:
			if instance.FileMod != nil {
				// Periodic flush of file output event buffer
				// Buffer might never fill and flush if we don't get enough messages
				instance.FileMod.FlushBuffer()
			}
		case msg, ok := <-popCh:
			func() {
				// Record panics and continue output
				defer func() {
					if fatalError := recover(); fatalError != nil {
						stack := debug.Stack()
						logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
							"panic in file output worker thread: %v\n%s", fatalError, stack)
					}
				}()

				if !ok {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
						"failed to retrieve waiting log message from assembler to output queue\n")
					return
				}
				instance.Metrics.ReceivedMessages.Add(1)

				// Write message to all outputs
				n, err := instance.FileMod.Write(ctx, msg)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"Failed to write message(s) to file output: %w\n", err)
				}
				instance.Metrics.SuccessfulFileWrites.Add(uint64(n))

				n, err = instance.JrnlMod.Write(ctx, msg)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"Failed to write message(s) to journald output: %w\n", err)
				}
				instance.Metrics.SuccessfulJrnlWrites.Add(uint64(n))

				n, err = instance.BeatsMod.Write(ctx, msg)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"Failed to write message(s) to beats output: %w\n", err)
				}
				instance.Metrics.SuccessfulBeatsWrites.Add(uint64(n))
			}()
		}
	}
}
