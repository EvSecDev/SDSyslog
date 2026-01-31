package beats

import (
	"context"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/pkg/protocol"
	"strings"
)

// Writes log message and associated metadata to configured beats server
func (mod *OutModule) Write(ctx context.Context, msg protocol.Payload) (logsSent int, err error) {
	if mod == nil {
		return
	}

	customFields := make(map[string]interface{})
	for key, value := range msg.CustomFields {
		key = strings.TrimPrefix(key, "_") // Remove journal internal fields prefix
		customFields[key] = value
	}

	fields := map[string]interface{}{
		// Minimum required fields
		"@timestamp": msg.Timestamp,
		"message":    string(msg.Data),

		// Common fields
		"host": map[string]interface{}{
			"name":     msg.Hostname,
			"hostname": msg.Hostname,
			"id":       msg.HostID,
			"ip":       msg.RemoteIP,
		},
		"agent": map[string]interface{}{
			"name": msg.Hostname, // Treated as remote host name for some parsers
			// Meta fields identifying sdsyslog daemon itself
			"program": global.ProgBaseName,
			"version": global.ProgVersion,
			"type":    "filebeat",
			"pid":     os.Getpid(),
		},

		// Custom fields written to syslog namespace
		"log": map[string]interface{}{
			"id":     msg.MsgID,
			"syslog": customFields,
		},
	}
	events := []interface{}{fields}

	logsSent, err = mod.sink.Send(events)
	if err != nil {
		return
	}
	return
}
