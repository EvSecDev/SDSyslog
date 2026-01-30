package journald

import (
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/syslog"
	"strconv"
	"time"
)

// Extracts relevant fields from a journal entry
func parseFields(fields map[string]string, localHostname string) (message global.ParsedMessage, err error) {
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
		err = fmt.Errorf("journal message priority '%s' is invalid: %v", journalPriority, err)
		return
	}
	message.Severity, err = syslog.CodeToSeverity(uint16(jrnlPriInt))
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
		message.ProcessID = os.Getpid()
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
	message.Facility, err = syslog.CodeToFacility(uint16(jrnlSeverityInt))
	if err != nil {
		err = fmt.Errorf("invalid severity '%d': %v", jrnlSeverityInt, err)
		return
	}
	return
}
