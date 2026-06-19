package beats

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/pkg/protocol"
	"strings"
	"syscall"

	lumberjack "github.com/elastic/go-lumber/client/v2"
)

// Writes log message and associated metadata to configured beats server
func (mod *OutModule) Write(ctx context.Context, msg *protocol.Payload) (logsSent int, err error) {
	if mod == nil {
		return
	}

	customFields := make(map[string]any)
	for key, value := range msg.CustomFields {
		key = strings.TrimPrefix(key, "_") // Remove journal internal fields prefix
		customFields[key] = value
	}

	fields := map[string]any{
		// Minimum required fields
		"@timestamp": msg.Timestamp,
		"message":    string(msg.Data),

		// Common fields
		"host": map[string]any{
			"name":     msg.Hostname,
			"hostname": msg.Hostname,
			"id":       msg.HostID,
			"ip":       msg.RemoteIP,
		},
		"agent": map[string]any{
			"name": msg.Hostname, // Treated as remote host name for some parsers
			// Meta fields identifying sdsyslog daemon itself
			"program": global.ProgBaseName,
			"version": global.ProgVersion,
			"type":    "filebeat",
			"pid":     os.Getpid(),
		},

		// Custom fields written to syslog namespace
		"log": map[string]any{
			"id":     msg.MsgID,
			"syslog": customFields,
		},
	}
	events := []any{fields}

	for range mod.maxSendRetries {
		logsSent, err = mod.sink.Send(events)
		if err != nil {
			// Retryable errors
			if errors.Is(err, syscall.EPIPE) ||
				errors.Is(err, os.ErrDeadlineExceeded) {
				// Re-open connection
				_ = mod.sink.Close()
				mod.sink, err = lumberjack.SyncDial(mod.endpoint, mod.compression, mod.timeout)
				if err != nil {
					err = fmt.Errorf("failed re-connection to beats server after remote ended the connection: %w", err)
					return
				}
				continue
			} else {
				// Fatal Error
				return
			}
		} else {
			break
		}
	}
	return
}

// No-op (for now) - satisfies common type
func (mod *OutModule) FlushBuffer() (flushedCnt int, err error) {
	return
}
