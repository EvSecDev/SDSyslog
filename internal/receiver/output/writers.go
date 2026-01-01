package output

import (
	"bytes"
	"context"
	"io"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Writes log message and associated metadata in one line to configured file
func writeFile(lineBuffer *[]string, msg protocol.Payload, file io.Writer) (err error) {
	newEntry := FormatAsText(msg)

	// Always ensure outputs have only one trailing newline
	var lineParts []string
	if !strings.HasSuffix(newEntry, "\n") {
		lineParts = append(lineParts, newEntry+"\n")
	} else {
		lineParts = []string{newEntry}
	}
	newLine := strings.Join(lineParts, " ")

	// Buffer small amount to reorder and write in batches
	*lineBuffer = append(*lineBuffer, newLine)

	// Batch 20 at a time
	if len(*lineBuffer) > 20 {
		err = flushFileBuffer(lineBuffer, file)
		if err != nil {
			return
		}
	}

	return
}

// Flushes line buffer to the file
func flushFileBuffer(lineBuffer *[]string, file io.Writer) (err error) {
	if lineBuffer == nil {
		return
	}

	if len(*lineBuffer) == 0 {
		return
	}

	sort.Slice(*lineBuffer, func(i, j int) bool {
		// Extract timestamp prefix (up to first space)
		getTime := func(s string) time.Time {
			ts := s
			if idx := strings.IndexByte(s, ' '); idx != -1 {
				ts = s[:idx]
			}
			t, err := time.Parse(time.RFC3339Nano, ts)
			if err != nil {
				return time.Time{} // zero time on error
			}
			return t
		}

		ti := getTime((*lineBuffer)[i])
		tj := getTime((*lineBuffer)[j])

		// Newest first, compare reverse
		return ti.After(tj)
	})

	for _, line := range *lineBuffer {
		data := []byte(line)
		for len(data) > 0 {
			var n int
			n, err = file.Write(data)
			if err != nil {
				return
			}
			data = data[n:] // remove the bytes that were successfully written
		}
	}

	// All writes succeeded, empty buffer
	*lineBuffer = []string{}

	return
}

// Writes log message and associated metadata to systemd journald
func writeJrnl(ctx context.Context, msg protocol.Payload, jrnl io.Writer) (err error) {
	severityInt, err := protocol.SeverityToCode(msg.Severity)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"Invalid severity %s: %v : message: host id %d, log id %d, hostname %s, application name %s\n",
			msg.Severity, err, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		severityInt = 6 // info
	}

	facilityInt, err := protocol.FacilityToCode(msg.Facility)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"Invalid facility %s: %v : message: host id %d, log id %d, hostname %s, application name %s\n",
			msg.Severity, err, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
		severityInt = 1 // user
	}

	pid := strconv.Itoa(msg.ProcessID)

	fields := map[string]string{
		"MESSAGE":           string(msg.LogText),
		"PRIORITY":          strconv.Itoa(int(severityInt)),
		"SYSLOG_FACILITY":   strconv.Itoa(int(facilityInt)),
		"SYSLOG_IDENTIFIER": msg.ApplicationName,
		"SYSLOG_PID":        pid,
		"OBJECT_PID":        pid,
		"_HOSTNAME":         msg.Hostname,
		"SYSLOG_TIMESTAMP":  msg.Timestamp.Format(time.RFC3339Nano),
	}

	var buf bytes.Buffer
	for k, v := range fields {
		if k == "" {
			continue
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
		buf.WriteByte('\n')
	}

	_, err = jrnl.Write(buf.Bytes())
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
			"Failed sending message to journal: %v : message: host id %d, log id %d, hostname %s, application name %s\n",
			err, msg.HostID, msg.LogID, msg.Hostname, msg.ApplicationName)
	}

	return
}
