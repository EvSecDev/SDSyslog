package journald

import (
	"bytes"
	"context"
	"fmt"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"strings"
	"time"
)

// Writes log message and associated metadata to systemd journald
func (mod *OutModule) Write(ctx context.Context, msg protocol.Payload) (entriesWritten int, err error) {
	if mod == nil {
		return
	}

	fields := map[string]string{
		"__REALTIME_TIMESTAMP": fmt.Sprintf("%d", time.Now().UnixMicro()), // Required field
		"_BOOT_ID":             global.BootID(),                           // Required field
		"MESSAGE":              string(msg.Data),                          // Required field
		"HOSTNAME":             msg.Hostname,
		"SYSLOG_HOSTNAME":      msg.Hostname,
		"SYSLOG_TIMESTAMP":     msg.Timestamp.Format(time.RFC3339Nano),
		"REMOTE_IP":            msg.RemoteIP,
	}
	for key, value := range msg.CustomFields {
		key = strings.TrimPrefix(key, "_") // Remove journal internal fields prefix before write

		text := protocol.FormatValue(value)
		if text == "" {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
				"invalid field type for key '%s'\n", key)
		} else {
			fields[key] = text
		}
	}

	// Key=val\n Format
	var buf bytes.Buffer
	for key, value := range fields {
		if key == "" || value == "" {
			continue
		}
		buf.WriteString(key)
		buf.WriteByte('=')
		buf.WriteString(value)
		buf.WriteByte('\n')
	}
	// Terminate with double newline
	buf.WriteByte('\n')

	err = sendJournalExport(mod.sink, mod.url, buf.Bytes())
	if err != nil {
		err = fmt.Errorf("%w (message: ip: '%s', host id '%d', message id '%d', hostname '%s')\n",
			err, msg.RemoteIP, msg.HostID, msg.MsgID, msg.Hostname)
		return
	}
	entriesWritten = 1

	return
}
