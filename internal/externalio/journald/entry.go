package journald

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"strconv"
	"time"
)

// Writes log message and associated metadata to systemd journald
func Write(ctx context.Context, msg protocol.Payload, jrnl *http.Client, jrnlURL string) (err error) {
	severityInt, err := protocol.SeverityToCode(msg.Severity)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"%v (message: host id '%d', log id '%d', hostname '%s', application name '%s')\n",
			err, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		severityInt = 6 // info
	}

	facilityInt, err := protocol.FacilityToCode(msg.Facility)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"%v (message: host id '%d', log id '%d', hostname '%s', application name '%s')\n",
			err, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		facilityInt = 1 // user
	}

	pid := strconv.Itoa(msg.ProcessID)

	// Build ordered list of fields
	type field struct {
		key string
		val string
	}
	fields := []field{
		{key: "__REALTIME_TIMESTAMP", val: fmt.Sprintf("%d", time.Now().UnixMicro())}, // Required field
		{key: "_BOOT_ID", val: global.BootID},                                         // Required field
		{key: "PRIORITY", val: strconv.Itoa(int(severityInt))},
		{key: "SYSLOG_IDENTIFIER", val: msg.ApplicationName},
		{key: "MESSAGE", val: string(msg.LogText)}, // Required field
		{key: "SYSLOG_FACILITY", val: strconv.Itoa(int(facilityInt))},
		{key: "SYSLOG_PID", val: pid},
		{key: "OBJECT_PID", val: pid},
		{key: "HOSTNAME", val: msg.Hostname},
		{key: "SYSLOG_HOSTNAME", val: msg.Hostname},
		{key: "SYSLOG_TIMESTAMP", val: msg.Timestamp.Format(time.RFC3339Nano)},
	}

	// Key=val\n Format
	var buf bytes.Buffer
	for _, field := range fields {
		if field.key == "" || field.val == "" {
			continue
		}
		buf.WriteString(field.key)
		buf.WriteByte('=')
		buf.WriteString(field.val)
		buf.WriteByte('\n')
	}
	// Terminate with double newline
	buf.WriteByte('\n')

	err = sendJournalExport(jrnl, jrnlURL, buf.Bytes())
	if err != nil {
		err = fmt.Errorf("%v (message: host id '%d', log id '%d', hostname '%s', application name '%s')\n",
			err, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		return
	}

	return
}
