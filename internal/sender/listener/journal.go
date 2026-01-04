package listener

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"strconv"
	"time"
)

// New creates a file listener instance
func NewJrnlSource(namespace []string, input io.ReadCloser, queue *mpmc.Queue[ParsedMessage], stateFilePath string) (new *JrnlInstance) {
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
			msg, err := extractFields(fields)
			if err != nil {
				if err == io.EOF {
					return
				}
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"field parse error: %v\n", err)
				return
			}
			pushBlocking(ctx, instance.Outbox, msg)
		}()
	}
}

// Extracts relevant fields from a journal entry
func extractFields(fields map[string]string) (message ParsedMessage, err error) {
	var ok bool

	// RAW LOG
	message.Text, ok = fields["MESSAGE"]
	if !ok {
		err = fmt.Errorf("journal entry has no message")
		return
	}

	// TIMESTAMP
	rawTimestamp, ok := fields["__REALTIME_TIMESTAMP"]
	if !ok {
		err = fmt.Errorf("journal entry has no realtime timestamp")
		return
	}
	rawTimestampUs, err := strconv.ParseInt(rawTimestamp, 0, 64)
	if err != nil {
		err = fmt.Errorf("failed parsing journal realtime timestamp: %v", err)
		return
	}
	message.Timestamp = time.Unix(
		int64(rawTimestampUs/1_000_000),         // seconds
		int64((rawTimestampUs%1_000_000)*1_000), // nanoseconds
	)

	// APPLICATION NAME
	candidates := []string{"SYSLOG_IDENTIFIER", "_SYSTEMD_USER_UNIT", "_SYSTEMD_UNIT"}
	for _, key := range candidates {
		if val, ok := fields[key]; ok {
			message.ApplicationName = val
			break
		}
	}
	if message.ApplicationName == "" {
		err = fmt.Errorf("journal entry has no unit field")
		return
	}

	// HOSTNAME
	message.Hostname, ok = fields["_HOSTNAME"]
	if !ok {
		message.Hostname = global.Hostname
	}

	// PRIORITY
	journalPriority, ok := fields["PRIORITY"]
	if !ok {
		err = fmt.Errorf("journal entry has no priority field")
		return
	}
	jrnlPriInt, err := strconv.Atoi(journalPriority)
	if err != nil {
		err = fmt.Errorf("journal message priority '%s' is invalid: %v", journalPriority, err)
		return
	}
	message.Severity, err = protocol.CodeToSeverity(uint16(jrnlPriInt))
	if err != nil {
		err = fmt.Errorf("invalid severity '%d': %v", jrnlPriInt, err)
		return
	}

	// PROCESS ID
	var pidStr string
	candidates = []string{"_PID", "SYSLOG_PID"}
	for _, key := range candidates {
		if val, ok := fields[key]; ok {
			pidStr = val
			break
		}
	}
	if pidStr != "" {
		message.ProcessID, err = strconv.Atoi(pidStr)
		if err != nil {
			err = fmt.Errorf("invalid pid '%s': %v", pidStr, err)
			return
		}
	} else {
		// Using self for missing pid
		message.ProcessID = global.PID
	}

	// FACILITY
	journalFacility, ok := fields["SYSLOG_FACILITY"]
	if !ok {
		journalFacility = "3" // Default to daemon
	}
	jrnlSeverityInt, err := strconv.Atoi(journalFacility)
	if err != nil {
		err = fmt.Errorf("journal message priority '%s' is invalid: %v", journalFacility, err)
		return
	}
	message.Facility, err = protocol.CodeToFacility(uint16(jrnlSeverityInt))
	if err != nil {
		err = fmt.Errorf("invalid severity '%d': %v", jrnlSeverityInt, err)
		return
	}
	return
}
