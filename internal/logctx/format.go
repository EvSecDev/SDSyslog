package logctx

import (
	"fmt"
	"strings"
	"time"
)

// Stringify full event
func (event Event) Format() (text string) {
	// Only print parts that are present
	var parts []string
	if !event.Timestamp.IsZero() {
		parts = append(parts, fmt.Sprintf("[%s]", padTimestamp(event.Timestamp)))
	}

	if len(event.Tags) > 0 {
		tagPrefixes := "["
		tagPrefixes += strings.Join(event.Tags, "/")
		tagPrefixes += "]"
		parts = append(parts, tagPrefixes)
	}

	if event.Severity != "" {
		parts = append(parts, fmt.Sprintf("[%s]", event.Severity))
	}

	if event.Message != "" {
		parts = append(parts, event.Message)
	}

	text = strings.Join(parts, " ")
	// No newline, message creator determines newlines
	return
}

// Ensures fixed length strings for timestamps
func padTimestamp(timestamp time.Time) (formatted string) {
	formatted = timestamp.Format(time.RFC3339Nano)

	majorFields := strings.Split(formatted, ".")
	if len(majorFields) != 2 {
		return
	}

	minorFields := strings.Split(majorFields[1], "-")
	if len(minorFields) != 2 {
		return
	}

	tsPrefix := majorFields[0]
	nanoseconds := minorFields[0]
	timezoneOffset := minorFields[1]

	// Pad the nanoseconds part to ensure it's 9 digits long
	for len(nanoseconds) < 9 {
		nanoseconds = "0" + nanoseconds
	}

	// Rebuild the timestamp with padded nanoseconds and the original timezone offset
	formatted = fmt.Sprintf("%s.%s-%s", tsPrefix, nanoseconds, timezoneOffset)
	return
}
