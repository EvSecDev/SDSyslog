package journald

import (
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/syslog"
	"sdsyslog/pkg/protocol"
	"strconv"
	"time"
)

// Extracts relevant fields from a journal entry
func parseFields(fields map[string]string, localHostname string) (message protocol.Message, err error) {
	var ok bool

	message.Fields = make(map[string]any)

	// RAW LOG
	message.Data, ok = fields["MESSAGE"]
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
		err = fmt.Errorf("failed parsing journal realtime timestamp: %w", err)
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
			message.Fields[global.CFappname] = val
			break
		}
	}
	_, ok = message.Fields[global.CFappname]
	if !ok {
		err = fmt.Errorf("journal entry has no unit field")
		return
	}

	// HOSTNAME
	message.Hostname, ok = fields["_HOSTNAME"]
	if !ok {
		message.Hostname = localHostname
	}

	// PRIORITY
	journalPriority, ok := fields["PRIORITY"]
	if !ok {
		err = fmt.Errorf("journal entry has no priority field")
		return
	}
	jrnlPriInt, err := strconv.Atoi(journalPriority)
	if err != nil {
		err = fmt.Errorf("journal message priority '%s' is invalid: %w", journalPriority, err)
		return
	}
	message.Fields[global.CFseverity], err = syslog.CodeToSeverity(uint16(jrnlPriInt))
	if err != nil {
		err = fmt.Errorf("invalid severity '%d': %w", jrnlPriInt, err)
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
		message.Fields[global.CFprocessid], err = strconv.Atoi(pidStr)
		if err != nil {
			err = fmt.Errorf("invalid pid '%s': %w", pidStr, err)
			return
		}
	} else {
		// Using self for missing pid
		message.Fields[global.CFprocessid] = os.Getpid()
	}

	// FACILITY
	journalFacility, ok := fields["SYSLOG_FACILITY"]
	if !ok {
		journalFacility = "3" // Default to daemon
	}
	jrnlSeverityInt, err := strconv.Atoi(journalFacility)
	if err != nil {
		err = fmt.Errorf("journal message priority '%s' is invalid: %w", journalFacility, err)
		return
	}
	message.Fields[global.CFfacility], err = syslog.CodeToFacility(uint16(jrnlSeverityInt))
	if err != nil {
		err = fmt.Errorf("invalid severity '%d': %w", jrnlSeverityInt, err)
		return
	}

	// Retrieve custom fields - best effort
	permittedJournalFields := []string{"_EXE", "_COMM", "_CMDLINE", "_UID", "_GID"}
	for _, field := range permittedJournalFields {
		jrnlValue, ok := fields["SYSLOG_FACILITY"]
		if !ok {
			continue
		}

		message.Fields[field] = jrnlValue
	}

	return
}
