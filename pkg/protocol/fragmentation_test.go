package protocol

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFragmentAndDefragment(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		input             Payload
		maxPayloadSize    int
		fixedProtocolSize int
		expectError       bool
		expectFragments   bool
	}{
		{
			name: "Valid fragmentation and defragmentation",
			input: Payload{
				HostID:          101,
				LogID:           555,
				Facility:        "auth",
				Severity:        "info",
				Timestamp:       now,
				ProcessID:       1234,
				ApplicationName: "testapp",
				Hostname:        "server1",
				LogText:         bytes.Repeat([]byte("This is a long message that will need to be fragmented into multiple packets."), 5),
			},
			maxPayloadSize:    100,
			fixedProtocolSize: 10,
			expectError:       false,
			expectFragments:   true,
		},
		{
			name: "Valid no frag",
			input: Payload{
				HostID:          1,
				LogID:           2,
				Facility:        "auth",
				Severity:        "info",
				Timestamp:       now,
				ProcessID:       10,
				ApplicationName: "testapp",
				Hostname:        "server1",
				LogText:         []byte("Short message."),
			},
			maxPayloadSize:    150,
			fixedProtocolSize: 1,
			expectError:       false,
			expectFragments:   false,
		},
		{
			name: "Valid large frag",
			input: Payload{
				HostID:          1,
				LogID:           2,
				Facility:        "auth",
				Severity:        "info",
				Timestamp:       now,
				ProcessID:       10,
				ApplicationName: "testapp",
				Hostname:        "server1",
				LogText:         []byte(strings.Repeat("a", 2500)),
			},
			maxPayloadSize:    1400,
			fixedProtocolSize: 250,
			expectError:       false,
			expectFragments:   true,
		},
		{
			name: "Invalid maxPayloadSize",
			input: Payload{
				LogText: []byte("test"),
			},
			maxPayloadSize:    0,
			fixedProtocolSize: 10,
			expectError:       true,
			expectFragments:   false,
		},
		{
			name: "Invalid protocolSize",
			input: Payload{
				LogText: []byte("test"),
			},
			maxPayloadSize:    100,
			fixedProtocolSize: 0,
			expectError:       true,
			expectFragments:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frags, err := Fragment(tt.input, tt.maxPayloadSize, tt.fixedProtocolSize)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			maxSeq := len(frags) - 1 // sequences are indexed off 0

			if tt.expectFragments && maxSeq <= 1 {
				t.Fatalf("Expected multiple fragments, got %d", maxSeq)
			}

			for _, f := range frags {
				if f.HostID != tt.input.HostID ||
					f.LogID != tt.input.LogID ||
					f.Facility != tt.input.Facility ||
					f.Severity != tt.input.Severity {
					t.Errorf("Shared field mismatch in fragment")
				}
				if f.MessageSeqMax != maxSeq {
					t.Errorf("Expected MessageSeqMax=%d, got %d", maxSeq, f.MessageSeqMax)
				}
			}

			reassembled, err := Defragment(frags)
			if err != nil {
				t.Fatalf("expected no error from defrag, but got: %v", err)
			}

			if !bytes.Equal(reassembled.LogText, tt.input.LogText) {
				t.Errorf("Reassembled text mismatch.\nGot:  %s\nWant: %s", reassembled.LogText, tt.input.LogText)
			}
		})
	}
}

func TestDefragment_ErrorsAndOrdering(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		input       []Payload
		expectError bool
		expectedLog []byte
	}{
		{
			name:        "Empty input slice",
			input:       []Payload{},
			expectError: true,
		},
		{
			name: "Mismatched shared fields",
			input: []Payload{
				{HostID: 1, LogID: 5, Facility: "auth", Severity: "info", Timestamp: now},
				{HostID: 2, LogID: 5, Facility: "auth", Severity: "info", Timestamp: now},
			},
			expectError: true,
		},
		{
			name: "Out-of-order fragments",
			input: []Payload{
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 1, MessageSeqMax: 1, LogText: []byte("world")},
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 0, MessageSeqMax: 1, LogText: []byte("hello ")},
			},
			expectError: false,
			expectedLog: []byte("hello world"),
		},
		{
			name: "Missing fragments beginning middle",
			input: []Payload{
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 1, MessageSeqMax: 3, LogText: []byte("second text")},
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 3, MessageSeqMax: 3, LogText: []byte("fourth text")},
			},
			expectError: false,
			expectedLog: []byte(missingLogPlaceholder + "second text" + missingLogPlaceholder + "fourth text"),
		},
		{
			name: "Missing fragments double middle",
			input: []Payload{
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 0, MessageSeqMax: 3, LogText: []byte("first text")},
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 3, MessageSeqMax: 3, LogText: []byte("fourth text")},
			},
			expectError: false,
			expectedLog: []byte("first text" + missingLogPlaceholder + missingLogPlaceholder + "fourth text"),
		},
		{
			name: "Missing fragments end",
			input: []Payload{
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 0, MessageSeqMax: 2, LogText: []byte("first text")},
				{HostID: 1, LogID: 99, Facility: "sys", Severity: "info", Timestamp: now, MessageSeq: 1, MessageSeqMax: 2, LogText: []byte(" second text")},
			},
			expectError: false,
			expectedLog: []byte("first text" + " second text" + missingLogPlaceholder),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reassembled, err := Defragment(tt.input)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !bytes.Equal(reassembled.LogText, tt.expectedLog) {
				t.Errorf("Expected reassembled text %q, got %q", tt.expectedLog, reassembled.LogText)
			}
		})
	}
}
