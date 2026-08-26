package output

import (
	"context"
	"os"
	"runtime/debug"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"syscall"
	"time"
)

// Creates new worker instance
func (manager *Manager) newWorker() (new *Instance) {
	new = &Instance{
		namespace: append(logctx.GetTagList(manager.ctx), logctx.NSWorker),
		inbox:     manager.Inbox,
		Metrics:   MetricStorage{},
		failures: failureTracker{
			maximumDuration: manager.Config.ConsecutiveFailureShutdownInterval,
		},
	}
	return
}

// Take assembled messages and write to configured outputs
func (instance *Instance) run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	popCh := make(chan *protocol.Payload, 1)

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
			if instance.outModules["file"] != nil {
				_, err := instance.outModules["file"].FlushBuffer()
				if err != nil {
					logctx.LogStdErr(ctx,
						"failed to flush file line buffer to disk: %w\n", err)
				}
			}
			return
		case <-ticker.C:
			if instance.outModules["file"] != nil {
				// Periodic flush of file output event buffer
				// Buffer might never fill and flush if we don't get enough messages
				_, err := instance.outModules["file"].FlushBuffer()
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
				var totalWritten int
				for moduleName, module := range instance.outModules {
					bytesWritten, err := module.Write(ctx, msg)
					if err != nil {
						logctx.LogStdErr(ctx,
							"Failed to write message(s) to %s output: %w\n", moduleName, err)
					} else {
						instance.writeModuleMetrics(moduleName, bytesWritten)
						totalWritten += bytesWritten
					}
				}

				// Record consecutive total failures
				if totalWritten == 0 {
					instance.Metrics.Dropped.Add(1)

					// Initialize deadline
					if instance.failures.consecutiveCount == 0 {
						instance.failures.deadline = time.Now().Add(instance.failures.maximumDuration)
					}

					// Increment total counter
					instance.failures.consecutiveCount++
				} else if instance.failures.consecutiveCount > 0 {
					// Reset consecutive fails
					instance.failures.consecutiveCount = 0
				}

				if instance.failures.consecutiveCount > 0 && time.Now().After(instance.failures.deadline) {
					logctx.LogStdFatal(ctx, "All outputs have failed writes a total of %d times over %s. Program will shutdown now.\n",
						instance.failures.consecutiveCount, instance.failures.maximumDuration.String())

					// Long term output failures means our own logs about output failures would go unnoticed
					// Stop entire program for better visibility into fatal conditions like this
					// Using OS signals to conduct the graceful shutdown through the signal handler in lifecycle
					err := syscall.Kill(os.Getpid(), syscall.SIGTERM)
					if err != nil {
						logctx.LogStdFatal(ctx, "Failed to issue SIGTERM to self process after fatal amount of output write failures.\n")
					}

					// Continue trying outputs, either signal handler gracefully shuts the daemon down, or we continue trying for eternity
				}
			}()
		}
	}
}
