package protocol

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestConstructPayload(t *testing.T) {
	InitBidiMaps()

	testCases := []struct {
		name     string
		input    Payload
		expected InnerWireFormat
		err      string
	}{
		{
			name: "valid payload",
			input: Payload{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        "user",
				Severity:        "info",
				ProcessID:       1234,
				Hostname:        "test-host",
				ApplicationName: "app1",
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: InnerWireFormat{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        1, // "user" maps to 1
				Severity:        6, // "info" maps to 6
				ProcessID:       1234,
				Hostname:        []byte("test-host"),
				ApplicationName: []byte("app1"),
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			err: "",
		},
		{
			name: "messageSeq larger than messageSeqMax",
			input: Payload{
				HostID:          1,
				LogID:           2,
				MessageSeq:      5,
				MessageSeqMax:   4, // Invalid case
				Facility:        "user",
				Severity:        "info",
				ProcessID:       1234,
				Hostname:        "test-host",
				ApplicationName: "app1",
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: InnerWireFormat{},
			err:      "message sequence cannot be larger than maximum sequence",
		},
		{
			name: "invalid facility",
			input: Payload{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        "invalid-facility", // Invalid facility
				Severity:        "info",
				ProcessID:       1234,
				Hostname:        "test-host",
				ApplicationName: "app1",
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: InnerWireFormat{},
			err:      "invalid facility: unknown facility name: invalid-facility",
		},
		{
			name: "invalid severity",
			input: Payload{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        "user",
				Severity:        "invalid-severity", // Invalid severity
				ProcessID:       1234,
				Hostname:        "test-host",
				ApplicationName: "app1",
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: InnerWireFormat{},
			err:      "invalid severity: unknown severity name: invalid-severity",
		},
		{
			name: "invalid padding length",
			input: Payload{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        "user",
				Severity:        "info",
				ProcessID:       1234,
				Hostname:        "test-host",
				ApplicationName: "app1",
				LogText:         []byte("log message"),
				PaddingLen:      500, // Invalid padding length
			},
			expected: InnerWireFormat{},
			err:      fmt.Sprintf("invalid padding length 500: must be between %d and %d", minPaddingLen, maxPaddingLen),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ttTime := time.Now()
			tt.input.Timestamp = ttTime
			tt.expected.Timestamp = uint64(ttTime.UnixMilli())

			proto, err := ValidatePayload(tt.input)
			if tt.err != "" {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if err.Error() != tt.err {
					t.Errorf("expected error '%v' but got '%v'", tt.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: '%v'", err)
				}
				if !compareProtocols(proto, tt.expected) {
					t.Errorf("expected '%v' but got '%v'", tt.expected, proto)
				}
			}
		})
	}
}

func TestDeconstructPayload(t *testing.T) {
	InitBidiMaps()

	testCases := []struct {
		name     string
		input    InnerWireFormat
		expected Payload
		err      string
	}{
		{
			name: "valid protocol",
			input: InnerWireFormat{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        1, // "user" maps to 1
				Severity:        6, // "info" maps to 6
				Timestamp:       uint64(time.Now().UnixMilli()),
				ProcessID:       1234,
				Hostname:        []byte("test-host"),
				ApplicationName: []byte("app1"),
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: Payload{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        "user",
				Severity:        "info",
				Timestamp:       time.UnixMilli(int64(time.Now().UnixMilli())),
				ProcessID:       1234,
				Hostname:        "test-host",
				ApplicationName: "app1",
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			err: "",
		},
		{
			name: "empty host ID",
			input: InnerWireFormat{
				HostID:          0, // Invalid HostID
				LogID:           1,
				MessageSeq:      1,
				MessageSeqMax:   1,
				Facility:        1,
				Severity:        6,
				Timestamp:       1000,
				ProcessID:       1234,
				Hostname:        []byte("test-host"),
				ApplicationName: []byte("app1"),
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: Payload{},
			err:      "empty host ID",
		},
		{
			name: "empty log ID",
			input: InnerWireFormat{
				HostID:          1,
				LogID:           0, // Invalid LogID
				MessageSeq:      1,
				MessageSeqMax:   1,
				Facility:        1,
				Severity:        6,
				Timestamp:       1000,
				ProcessID:       1234,
				Hostname:        []byte("test-host"),
				ApplicationName: []byte("app1"),
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: Payload{},
			err:      "empty log ID",
		},
		{
			name: "invalid facility code",
			input: InnerWireFormat{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        9999, // Invalid facility code
				Severity:        6,
				Timestamp:       uint64(time.Now().UnixMilli()),
				ProcessID:       1234,
				Hostname:        []byte("test-host"),
				ApplicationName: []byte("app1"),
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: Payload{},
			err:      "invalid facility: unknown facility code: 9999",
		},
		{
			name: "invalid severity code",
			input: InnerWireFormat{
				HostID:          1,
				LogID:           2,
				MessageSeq:      3,
				MessageSeqMax:   4,
				Facility:        1,
				Severity:        9999, // Invalid severity code
				Timestamp:       uint64(time.Now().UnixMilli()),
				ProcessID:       1234,
				Hostname:        []byte("test-host"),
				ApplicationName: []byte("app1"),
				LogText:         []byte("log message"),
				PaddingLen:      16,
			},
			expected: Payload{},
			err:      "invalid severity: unknown severity code: 9999",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			request, err := ParsePayload(tt.input)
			if tt.err != "" {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if err.Error() != tt.err {
					t.Errorf("expected error '%v' but got '%v'", tt.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: '%v'", err)
				}
				if !comparePayloads(request, tt.expected) {
					t.Errorf("expected '%v' but got '%v'", tt.expected, request)
				}
			}
		})
	}
}

// Helper function to compare Protocol values
func compareProtocols(p1, p2 InnerWireFormat) bool {
	return p1.HostID == p2.HostID &&
		p1.LogID == p2.LogID &&
		p1.MessageSeq == p2.MessageSeq &&
		p1.MessageSeqMax == p2.MessageSeqMax &&
		p1.Facility == p2.Facility &&
		p1.Severity == p2.Severity &&
		p1.Timestamp == p2.Timestamp &&
		p1.ProcessID == p2.ProcessID &&
		string(p1.Hostname) == string(p2.Hostname) &&
		string(p1.ApplicationName) == string(p2.ApplicationName) &&
		string(p1.LogText) == string(p2.LogText) &&
		p1.PaddingLen == p2.PaddingLen
}

// Helper function to compare Payload values
func comparePayloads(p1, p2 Payload) bool {
	return p1.HostID == p2.HostID &&
		p1.LogID == p2.LogID &&
		p1.MessageSeq == p2.MessageSeq &&
		p1.MessageSeqMax == p2.MessageSeqMax &&
		p1.Facility == p2.Facility &&
		p1.Severity == p2.Severity &&
		p1.Timestamp.Equal(p2.Timestamp) &&
		p1.ProcessID == p2.ProcessID &&
		p1.Hostname == p2.Hostname &&
		p1.ApplicationName == p2.ApplicationName &&
		bytes.Equal(p1.LogText, p2.LogText) &&
		p1.PaddingLen == p2.PaddingLen
}
