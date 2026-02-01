package journald

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"strings"
)

func (mod *InModule) Reader(ctx context.Context) {
	reader := bufio.NewReader(mod.sink)

	var localHostname string
	var iter uint64
	const refreshMask = 1024 - 1
	localHostname, err := os.Hostname()
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to retrieve current local hostname: %w\n", err)
		localHostname = "-"
		err = nil
	}

	var readPosition string
	for {
		select {
		case <-ctx.Done():
			err := savePosition(readPosition, mod.stateFile)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed to save position in journal source: %w\n", err)
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
					"error reading journal output: %w\n", err)
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
			msg, err := parseFields(fields, localHostname)
			if err != nil {
				if err == io.EOF {
					return
				}
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"field parse error: %w\n", err)
				return
			}
			mod.metrics.LinesRead.Add(1)

			msg.Fields[global.IOCtxKey] = strings.Join(mod.Namespace, "/")

			mod.outbox.PushBlocking(ctx, msg, msg.Size())
			mod.metrics.Success.Add(1)

			// Local hostname periodic refresh
			iter++
			if iter&refreshMask == 0 {
				newName, err := os.Hostname()
				if err == nil && newName != localHostname {
					localHostname = newName
				} else if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to refresh current local hostname: %w\n", err)
				}
			}
		}()
	}
}
