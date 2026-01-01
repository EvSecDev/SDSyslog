package output

import (
	"sdsyslog/pkg/protocol"
	"strconv"
	"strings"
	"time"
)

// Main raw log line format for outputs
// Fmt: '2020-01-01T10:10:10.123456789Z Server01 MyApp[1234]: Daemon: [INFO]: this is a log message'
func FormatAsText(msg protocol.Payload) (text string) {
	text =
		msg.Timestamp.Format(time.RFC3339Nano) + " " +
			msg.Hostname + " " +
			msg.ApplicationName +
			"[" + strconv.Itoa(msg.ProcessID) + "]: " +
			msg.Facility + ": " +
			"[" + strings.ToUpper(msg.Severity) + "]: " +
			string(msg.LogText)
	return
}
