package journald

import (
	"sdsyslog/internal/global"
	"sdsyslog/pkg/protocol"
	"strconv"
	"testing"
	"time"
)

func TestParseFields(t *testing.T) {
	protocol.InitBidiMaps()
	baseTimestampUs := int64(1_700_000_000_123_456)
	expectedTime := time.Unix(
		baseTimestampUs/1_000_000,
		(baseTimestampUs%1_000_000)*1_000,
	)

	tests := []struct {
		name        string
		input       map[string]string
		expected    global.ParsedMessage
		expectedErr bool
	}{
		{
			name: "basic valid entry",
			input: map[string]string{
				"MESSAGE":              "hello world",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"_HOSTNAME":            "test-host",
				"PRIORITY":             "6",
				"_PID":                 "1234",
				"SYSLOG_FACILITY":      "3",
			},
			expected: global.ParsedMessage{
				Text:            "hello world",
				ApplicationName: "my-app",
				Hostname:        "test-host",
				ProcessID:       1234,
				Timestamp:       expectedTime,
				Facility:        "daemon",
				Severity:        "info",
			},
		},
		{
			name: "missing MESSAGE",
			input: map[string]string{
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "6",
			},
			expectedErr: true,
		},
		{
			name: "missing timestamp",
			input: map[string]string{
				"MESSAGE":           "hello",
				"SYSLOG_IDENTIFIER": "my-app",
				"PRIORITY":          "6",
			},
			expectedErr: true,
		},
		{
			name: "invalid timestamp",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": "not-a-number",
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "6",
			},
			expectedErr: true,
		},
		{
			name: "application name fallback order",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"_SYSTEMD_USER_UNIT":   "user.service",
				"_SYSTEMD_UNIT":        "system.service",
				"PRIORITY":             "5",
			},
			expected: global.ParsedMessage{
				Text:            "hello",
				ApplicationName: "user.service",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       expectedTime,
				Facility:        "daemon",
				Severity:        "notice",
			},
		},
		{
			name: "missing application name",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"PRIORITY":             "6",
			},
			expectedErr: true,
		},
		{
			name: "hostname fallback",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "6",
			},
			expected: global.ParsedMessage{
				Text:            "hello",
				ApplicationName: "my-app",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       expectedTime,
				Facility:        "daemon",
				Severity:        "info",
			},
		},
		{
			name: "pid fallback to global PID",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "6",
			},
			expected: global.ParsedMessage{
				Text:            "hello",
				ApplicationName: "my-app",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       expectedTime,
				Facility:        "daemon",
				Severity:        "info",
			},
		},
		{
			name: "invalid pid",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "6",
				"_PID":                 "abc",
			},
			expectedErr: true,
		},
		{
			name: "invalid priority",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "not-a-number",
			},
			expectedErr: true,
		},
		{
			name: "invalid facility",
			input: map[string]string{
				"MESSAGE":              "hello",
				"__REALTIME_TIMESTAMP": strconv.FormatInt(baseTimestampUs, 10),
				"SYSLOG_IDENTIFIER":    "my-app",
				"PRIORITY":             "6",
				"SYSLOG_FACILITY":      "not-a-number",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := parseFields(tt.input)
			if err != nil && !tt.expectedErr {
				t.Fatalf("expected no error, but got '%s'", err)
			}
			if err == nil && tt.expectedErr {
				t.Fatalf("expected error, but got no error")
			}
			if err != nil && tt.expectedErr {
				return
			}

			if tt.expected.Text != msg.Text {
				t.Fatalf("expected Text '%s', but got '%s'", tt.expected.Text, msg.Text)
			}
			if tt.expected.ApplicationName != msg.ApplicationName {
				t.Fatalf("expected ApplicationName '%s', but got '%s'", tt.expected.ApplicationName, msg.ApplicationName)
			}
			if tt.expected.Hostname != msg.Hostname {
				t.Fatalf("expected Hostname '%s', but got '%s'", tt.expected.Hostname, msg.Hostname)
			}
			if tt.expected.ProcessID != msg.ProcessID {
				t.Fatalf("expected ProcessID '%d', but got '%d'", tt.expected.ProcessID, msg.ProcessID)
			}
			if tt.expected.Timestamp != msg.Timestamp {
				t.Fatalf("expected Timestamp '%s', but got '%s'", tt.expected.Timestamp, msg.Timestamp)
			}
			if tt.expected.Facility != msg.Facility {
				t.Fatalf("expected Facility '%s', but got '%s'", tt.expected.Facility, msg.Facility)
			}
			if tt.expected.Severity != msg.Severity {
				t.Fatalf("expected Severity '%s', but got '%s'", tt.expected.Severity, msg.Severity)
			}
		})
	}
}
