package output

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"time"
)

// Creates new worker instance
func (manager *Manager) newWorker() (new *Instance) {
	new = &Instance{
		namespace: append(logctx.GetTagList(manager.ctx), logctx.NSWorker),
		inbox:     manager.Inbox,
		Metrics:   MetricStorage{},
	}
	return
}

// Take assembled messages and write to configured outputs
func (instance *Instance) run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	popCh := make(chan protocol.Payload, 1)

	go func() {
		for {
			msg, ok := instance.inbox.Pop(ctx)
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
			atomics.Subtract(&instance.inbox.ActiveWrite.Load().Metrics.Bytes, uint64(size), 4)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			if instance.fileMod != nil {
				_, err := instance.fileMod.FlushBuffer()
				if err != nil {
					logctx.LogStdErr(ctx,
						"failed to flush file line buffer to disk: %w\n", err)
				}
			}
			return
		case <-ticker.C:
			if instance.fileMod != nil {
				// Periodic flush of file output event buffer
				// Buffer might never fill and flush if we don't get enough messages
				_, err := instance.fileMod.FlushBuffer()
				if err != nil {
					logctx.LogStdErr(ctx,
						"failed to flush file line buffer to disk: %w\n", err)
				}
			}
		case msg, ok := <-popCh:
			func() {
				// Record panics and continue output
				defer func() {
					if fatalError := recover(); fatalError != nil {
						stack := debug.Stack()
						logctx.LogStdErr(ctx,
							"panic in file output worker thread: %v\n%s", fatalError, stack)
					}
				}()

				if !ok {
					logctx.LogStdWarn(ctx,
						"failed to retrieve waiting log message from assembler to output queue\n")
					return
				}
				instance.Metrics.ReceivedMessages.Add(1)

				// Write message to all outputs
				n, err := instance.fileMod.Write(ctx, msg)
				if err != nil {
					logctx.LogStdErr(ctx,
						"Failed to write message(s) to file output: %w\n", err)
				}
				instance.Metrics.SuccessfulFileWrites.Add(uint64(n))

				n, err = instance.jrnlMod.Write(ctx, msg)
				if err != nil {
					logctx.LogStdErr(ctx,
						"Failed to write message(s) to journald output: %w\n", err)
				}
				instance.Metrics.SuccessfulJrnlWrites.Add(uint64(n))

				n, err = instance.beatsMod.Write(ctx, msg)
				if err != nil {
					logctx.LogStdErr(ctx,
						"Failed to write message(s) to beats output: %w\n", err)
				}
				instance.Metrics.SuccessfulBeatsWrites.Add(uint64(n))

				n, err = instance.rawMod.Write(ctx, msg)
				if err != nil {
					logctx.LogStdErr(ctx,
						"Failed to write message(s) to raw output: %w\n", err)
				}
				instance.Metrics.SuccessfulRawWrites.Add(uint64(n))
			}()
		}
	}
}
