package journald

import (
	"bufio"
	"context"
	"io"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
)

func (mod *InModule) Reader(ctx context.Context) {
	reader := bufio.NewReader(mod.sink)

	var readPosition string
	for {
		select {
		case <-ctx.Done():
			err := savePosition(readPosition, mod.stateFile)
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
			fields, err := extractEntry(reader)
			if err != nil {
				if err.Error() == "encountered empty entry" && ctx.Err() != nil {
					// Shutdown
					return
				}
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"error reading journal output: %v\n", err)
				return
			}
			if len(fields) == 0 {
				return
			}

			// Mark current cursor after successful entry retrieval
			var fieldPresent bool
			readPosition, fieldPresent = fields["__CURSOR"]
			if !fieldPresent {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed cursor extraction\n")
			}

			// Parse and retrieve fields we need
			msg, err := parseFields(fields)
			if err != nil {
				if err == io.EOF {
					return
				}
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"field parse error: %v\n", err)
				return
			}
			mod.metrics.LinesRead.Add(1)

			size := len(msg.Text) +
				len(msg.ApplicationName) +
				len(msg.Hostname) +
				len(msg.Facility) +
				len(msg.Severity) +
				16 // int64 size pid and time
			mod.outbox.PushBlocking(ctx, msg, size)
			mod.metrics.Success.Add(1)
		}()
	}
}
