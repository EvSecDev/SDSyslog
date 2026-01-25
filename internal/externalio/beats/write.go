package beats

import (
	"context"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
)

// Writes log message and associated metadata to configured beats server
func (mod *OutModule) Write(ctx context.Context, msg protocol.Payload) (logsSent int, err error) {
	if mod == nil {
		return
	}

	severityInt, err := protocol.SeverityToCode(msg.Severity)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"%v (message: ip: '%s', host id '%d', log id '%d', hostname '%s', application name '%s')\n",
			err, msg.RemoteIP, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		severityInt = 6 // info
	}

	facilityInt, err := protocol.FacilityToCode(msg.Facility)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"%v (message: ip: '%s', host id '%d', log id '%d', hostname '%s', application name '%s')\n",
			err, msg.RemoteIP, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		facilityInt = 1 // user
	}

	fields := map[string]interface{}{
		// Minimum required fields
		"@timestamp": msg.Timestamp,
		"message":    string(msg.LogText),

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
		"process": map[string]interface{}{
			"pid": msg.ProcessID,
		},

		// Syslog compat fields
		"log": map[string]interface{}{
			"id": msg.LogID, // Custom
			"syslog": map[string]interface{}{
				"appname": msg.ApplicationName,
				"facility": map[string]interface{}{
					"code": facilityInt,
					"name": msg.Facility,
				},
				"priority":      severityInt,
				"priority-name": msg.Severity,
			},
		},
	}
	events := []interface{}{fields}

	logsSent, err = mod.sink.Send(events)
	if err != nil {
		return
	}
	return
}
