package listener

import (
	"bufio"
	"context"
	"io"
	"runtime/debug"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
)

// New creates a file listener instance
func NewJrnlSource(namespace []string, input io.ReadCloser, queue *mpmc.Queue[global.ParsedMessage], stateFilePath string) (new *JrnlInstance) {
	new = &JrnlInstance{
		Namespace: append(namespace, global.NSoJrnl),
		Journal:   input,
		StateFile: stateFilePath,
		Outbox:    queue,
		Metrics:   &MetricStorage{},
	}
	return
}

func (instance *JrnlInstance) Run(ctx context.Context) {
	reader := bufio.NewReader(instance.Journal)

	var readPosition string
	for {
		select {
		case <-ctx.Done():
			err := journald.SavePosition(readPosition, instance.StateFile)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed to save position in journal source: %v\n", err)
			}
			return
		default:
		}

		func() {
			// Record panics and continue working
			defer func() {
				if fatalError := recover(); fatalError != nil {
					stack := debug.Stack()
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"panic in journal reader worker thread: %v\n%s", fatalError, stack)
				}
			}()

			var err error

			// Grab an entry from journal
			fields, err := journald.ExtractEntry(reader)
			if err != nil {
				if err.Error() == "encountered empty entry" && ctx.Err() != nil {
					// Shutdown
					return
				}
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"error reading journal output: %v\n", err)
				return
			}

			// Mark current cursor after successful entry retrieval
			var fieldPresent bool
			readPosition, fieldPresent = fields["__CURSOR"]
			if !fieldPresent {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed cursor extraction: %v\n", err)
			}

			// Parse and retrieve fields we need
			msg, err := journald.ParseFields(fields)
			if err != nil {
				if err == io.EOF {
					return
				}
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"field parse error: %v\n", err)
				return
			}
			instance.Metrics.LinesRead.Add(1)

			size := len(msg.Text) +
				len(msg.ApplicationName) +
				len(msg.Hostname) +
				len(msg.Facility) +
				len(msg.Severity) +
				16 // int64 size pid and time
			instance.Outbox.PushBlocking(ctx, msg, size)
			instance.Metrics.Success.Add(1)
		}()
	}
}
