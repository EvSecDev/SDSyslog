package journald

import (
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/syslog"
	"sdsyslog/pkg/protocol"
	"strconv"
	"testing"
	"time"
)

func TestParseFields(t *testing.T) {
	syslog.InitBidiMaps()
	baseTimestampUs := int64(1_700_000_000_123_456)
	expectedTime := time.Unix(
		baseTimestampUs/1_000_000,
		(baseTimestampUs%1_000_000)*1_000,
	)

	localHostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("failed to determine local hostname: %v", err)
	}

	tests := []struct {
		name        string
		input       map[string]string
		expected    protocol.Message
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
			expected: protocol.Message{
				Data:      "hello world",
				Hostname:  "test-host",
				Timestamp: expectedTime,
				Fields: map[string]any{
					global.CFappname:   "my-app",
					global.CFprocessid: 1234,
					global.CFfacility:  "daemon",
					global.CFseverity:  "info",
				},
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
			expected: protocol.Message{
				Data:      "hello",
				Hostname:  localHostname,
				Timestamp: expectedTime,
				Fields: map[string]any{
					global.CFappname:   "user.service",
					global.CFprocessid: os.Getpid(),
					global.CFfacility:  "daemon",
					global.CFseverity:  "notice",
				},
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
			expected: protocol.Message{
				Data:      "hello",
				Hostname:  localHostname,
				Timestamp: expectedTime,
				Fields: map[string]any{
					global.CFappname:   "my-app",
					global.CFprocessid: os.Getpid(),
					global.CFfacility:  "daemon",
					global.CFseverity:  "info",
				},
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
			expected: protocol.Message{
				Data:      "hello",
				Hostname:  localHostname,
				Timestamp: expectedTime,
				Fields: map[string]any{
					global.CFappname:   "my-app",
					global.CFprocessid: os.Getpid(),
					global.CFfacility:  "daemon",
					global.CFseverity:  "info",
				},
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
			msg, err := parseFields(tt.input, localHostname)
			if err != nil && !tt.expectedErr {
				t.Fatalf("expected no error, but got '%s'", err)
			}
			if err == nil && tt.expectedErr {
				t.Fatalf("expected error, but got no error")
			}
			if err != nil && tt.expectedErr {
				return
			}

			if tt.expected.Data != msg.Data {
				t.Fatalf("expected Data '%s', but got '%s'", tt.expected.Data, msg.Data)
			}
			if tt.expected.Hostname != msg.Hostname {
				t.Fatalf("expected Hostname '%s', but got '%s'", tt.expected.Hostname, msg.Hostname)
			}
			if tt.expected.Timestamp != msg.Timestamp {
				t.Fatalf("expected Timestamp '%s', but got '%s'", tt.expected.Timestamp, msg.Timestamp)
			}

			expected := tt.expected.Fields[global.CFappname]
			got, ok := msg.Fields[global.CFappname]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFappname)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFappname, expected, got)
			}

			expected = tt.expected.Fields[global.CFfacility]
			got, ok = msg.Fields[global.CFfacility]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFfacility)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFfacility, expected, got)
			}

			expected = tt.expected.Fields[global.CFprocessid]
			got, ok = msg.Fields[global.CFprocessid]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFprocessid)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFprocessid, expected, got)
			}

			expected = tt.expected.Fields[global.CFseverity]
			got, ok = msg.Fields[global.CFseverity]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFseverity)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFseverity, expected, got)
			}
		})
	}
}
