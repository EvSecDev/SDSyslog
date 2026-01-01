package protocol

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestConstructAndDeconstruct(t *testing.T) {
	tests := []struct {
		name        string
		input       InnerWireFormat
		expectedErr bool
	}{
		{
			name: "Normal",
			input: InnerWireFormat{
				HostID:          12345,
				LogID:           92789,
				MessageSeq:      1,
				MessageSeqMax:   5,
				Facility:        22,
				Severity:        5,
				Timestamp:       uint64(time.Now().Unix()),
				ProcessID:       9876,
				Hostname:        []byte("localhost"),
				ApplicationName: []byte("testApp"),
				LogText:         []byte("This is a test log message"),
				PaddingLen:      18,
			},
			expectedErr: false,
		},
		{
			name: "LongEven",
			input: InnerWireFormat{
				HostID:          12345,
				LogID:           67819,
				MessageSeq:      1,
				MessageSeqMax:   5,
				Facility:        22,
				Severity:        5,
				Timestamp:       uint64(time.Now().Unix()),
				ProcessID:       9876,
				Hostname:        []byte("localhost"),
				ApplicationName: []byte("testApp"),
				LogText:         []byte(strings.Repeat("t", 256)),
				PaddingLen:      28,
			},
			expectedErr: false,
		},
		{
			name: "LongOdd",
			input: InnerWireFormat{
				HostID:          12345,
				LogID:           67839,
				MessageSeq:      1,
				MessageSeqMax:   5,
				Facility:        22,
				Severity:        5,
				Timestamp:       uint64(time.Now().Unix()),
				ProcessID:       9876,
				Hostname:        []byte("localhost"),
				ApplicationName: []byte("testApp"),
				LogText:         []byte(strings.Repeat("t", 550)),
				PaddingLen:      38,
			},
			expectedErr: false,
		},
		{
			name: "Short",
			input: InnerWireFormat{
				HostID:          1,
				LogID:           1,
				MessageSeq:      1,
				MessageSeqMax:   5,
				Facility:        2,
				Severity:        5,
				Timestamp:       uint64(time.Now().Unix()),
				ProcessID:       1,
				Hostname:        []byte{},
				ApplicationName: []byte("-"),
				LogText:         []byte("t"),
				PaddingLen:      1,
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			serialized, err := ConstructInnerPayload(tt.input)
			if err != nil {
				t.Fatalf("Error serializing payload: %v", err)
			}

			// Deserialize
			deserialized, err := DeconstructInnerPayload(serialized)
			if tt.expectedErr && err == nil {
				t.Fatalf("deserializing: expected error, but got no error")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("deserializing: expected no error, but got '%v'", err)
			}
			if tt.expectedErr && err != nil {
				// do not check further for expected errors
				return
			}
			if err != nil {
				t.Fatalf("Error deserializing payload: %v", err)
			}

			// Validate fields
			if deserialized.HostID != tt.input.HostID {
				t.Errorf("HostID: expected %d, got %d", tt.input.HostID, deserialized.HostID)
			}
			if deserialized.LogID != tt.input.LogID {
				t.Errorf("LogID: expected %d, got %d", tt.input.LogID, deserialized.LogID)
			}
			if deserialized.MessageSeq != tt.input.MessageSeq {
				t.Errorf("MessageSeq: expected %d, got %d", tt.input.MessageSeq, deserialized.MessageSeq)
			}
			if deserialized.MessageSeqMax != tt.input.MessageSeqMax {
				t.Errorf("MessageSeqMax: expected %d, got %d", tt.input.MessageSeqMax, deserialized.MessageSeqMax)
			}
			if deserialized.Facility != tt.input.Facility {
				t.Errorf("Facility: expected %d, got %d", tt.input.Facility, deserialized.Facility)
			}
			if deserialized.Severity != tt.input.Severity {
				t.Errorf("Severity: expected %d, got %d", tt.input.Severity, deserialized.Severity)
			}
			if deserialized.Timestamp != tt.input.Timestamp {
				t.Errorf("Timestamp: expected %d, got %d", tt.input.Timestamp, deserialized.Timestamp)
			}
			if deserialized.ProcessID != tt.input.ProcessID {
				t.Errorf("ProcessID: expected %d, got %d", tt.input.ProcessID, deserialized.ProcessID)
			}
			if !bytes.Equal(deserialized.Hostname, tt.input.Hostname) {
				t.Errorf("Hostname: expected %v, got %v", tt.input.Hostname, deserialized.Hostname)
			}
			if !bytes.Equal(deserialized.ApplicationName, tt.input.ApplicationName) {
				t.Errorf("ApplicationName: expected %v, got %v", tt.input.ApplicationName, deserialized.ApplicationName)
			}
			if !bytes.Equal(deserialized.LogText, tt.input.LogText) {
				t.Errorf("LogText: expected %v, got %v", tt.input.LogText, deserialized.LogText)
			}
			if deserialized.PaddingLen != tt.input.PaddingLen {
				t.Errorf("PaddingLen: expected %d, got %d", tt.input.PaddingLen, deserialized.PaddingLen)
			}
		})
	}
}

func TestShortPayload(t *testing.T) {
	// Test deserialization with a payload that's too short
	payload := []byte("short")
	_, err := DeconstructInnerPayload(payload)
	if err == nil {
		t.Error("Expected error for short payload, but got none")
	}
}

func TestLongPayload(t *testing.T) {
	// Test serialization with a payload exceeding length field
	fields := InnerWireFormat{
		HostID:          12345,
		LogID:           96789,
		MessageSeq:      1,
		MessageSeqMax:   5,
		Facility:        22,
		Severity:        5,
		Timestamp:       uint64(time.Now().Unix()),
		ProcessID:       9876,
		Hostname:        []byte("localhost"),
		ApplicationName: []byte("testApp"),
		LogText:         []byte(strings.Repeat("t", 65536)),
		PaddingLen:      8,
	}
	_, err := ConstructInnerPayload(fields)
	if err == nil {
		t.Error("Expected error for longer payload, but got none")
	}
}

func TestInvalidLogText(t *testing.T) {
	// Test deserialization with invalid LogText (empty)
	fields := InnerWireFormat{
		HostID:          12345,
		LogID:           96789,
		MessageSeq:      1,
		MessageSeqMax:   5,
		Facility:        22,
		Severity:        5,
		Timestamp:       uint64(time.Now().Unix()),
		ProcessID:       9876,
		Hostname:        []byte("localhost"),
		ApplicationName: []byte("testApp"),
		LogText:         []byte(""),
		PaddingLen:      8,
	}

	serialized, err := ConstructInnerPayload(fields)
	if err != nil {
		t.Errorf("Expected no error for construction, but got %v", err)
	}

	// Modify the LogText field to be empty
	deserialized, err := DeconstructInnerPayload(serialized)
	if err == nil {
		t.Error("Expected error for empty LogText, but got none")
	}

	if deserialized.LogText != nil {
		t.Errorf("Expected nil LogText, got %v", deserialized.LogText)
	}
}

func TestPaddingLen(t *testing.T) {
	// Test that padding is added correctly
	fields := InnerWireFormat{
		HostID:          12345,
		LogID:           96789,
		MessageSeq:      1,
		MessageSeqMax:   5,
		Facility:        22,
		Severity:        5,
		Timestamp:       uint64(time.Now().Unix()),
		ProcessID:       9876,
		Hostname:        []byte("localhost"),
		ApplicationName: []byte("testApp"),
		LogText:         []byte("This is a test log message"),
		PaddingLen:      8,
	}

	// Serialize the payload
	serialized, _ := ConstructInnerPayload(fields)

	// Calculate the expected length based on test case
	expectedLen := 0
	expectedLen += lenHostID                           // HostID (uint32)
	expectedLen += lenLogID                            // LogID (uint16)
	expectedLen += lenMsgSeq                           // MessageSeq (uint16)
	expectedLen += lenSeqMax                           // MessageSeqMax (uint16)
	expectedLen += lenFacility                         // Facility (uint16)
	expectedLen += lenSeverity                         // Severity (uint16)
	expectedLen += lenTimestamp                        // Timestamp (uint64)
	expectedLen += lenProcID                           // ProcessID (uint32)
	expectedLen += 1 + len(fields.Hostname) + 1        // Hostname length + Hostname + Null terminator
	expectedLen += 1 + len(fields.ApplicationName) + 1 // ApplicationName length + ApplicationName + Null terminator
	expectedLen += 1 + len(fields.LogText) + 2         // LogText length + LogText + Null terminator
	expectedLen += fields.PaddingLen                   // Padding

	// Check that padding is added
	if len(serialized) != expectedLen {
		t.Errorf("Padding length is incorrect. Expected %d, got %d", expectedLen, len(serialized))
	}
}

func TestInvalidNextLengthByte(t *testing.T) {
	fields := InnerWireFormat{
		HostID:          12345,
		LogID:           96789,
		MessageSeq:      1,
		MessageSeqMax:   5,
		Facility:        22,
		Severity:        5,
		Timestamp:       uint64(time.Now().Unix()),
		ProcessID:       9876,
		Hostname:        []byte("localhost"),
		ApplicationName: []byte("testApp"),
		LogText:         []byte("This is a test log message"),
		PaddingLen:      8,
	}

	serialized, err := ConstructInnerPayload(fields)
	if err != nil {
		t.Fatalf("Error serializing payload: %v", err)
	}

	// Make a copy to corrupt
	corrupted := make([]byte, len(serialized))
	copy(corrupted, serialized)

	// Find a position where a length byte likely resides.
	// Flip that byte to an unrealistic value
	for i := 0; i < len(corrupted)-len(fields.Hostname); i++ {
		if bytes.HasPrefix(corrupted[i+1:], fields.Hostname) {
			corrupted[i] = 255 // corrupt the "length" byte before hostname
			break
		}
	}

	// Try to deconstruct the corrupted payload
	_, err = DeconstructInnerPayload(corrupted)
	if err == nil {
		t.Error("Expected error when deconstructing payload with corrupted length byte, but got none")
	} else {
		t.Logf("Successfully detected corrupted payload: %v", err)
	}
}
