package protocol

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
	"time"
)

func TestConstructAndDeconstruct(t *testing.T) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, int64(1234))
	if err != nil {
		t.Fatalf("failed to mock integer: %v", err)
	}
	mockInt := buf.Bytes()

	tests := []struct {
		name        string
		input       innerWireFormat
		expectedErr bool
	}{
		{
			name: "Normal",
			input: innerWireFormat{
				HostID:        12345,
				MsgID:         92789,
				MessageSeq:    1,
				MessageSeqMax: 5,
				Timestamp:     uint64(time.Now().Unix()),
				Hostname:      []byte("localhost"),
				ContextFields: []contextWireFormat{
					{
						Key:     []byte("applicationname"),
						valType: ContextString,
						Value:   []byte("user"),
					},
					{
						Key:     []byte("facility"),
						valType: ContextInt64,
						Value:   mockInt,
					},
				},
				Data:       []byte("This is a test log message"),
				PaddingLen: 18,
			},
			expectedErr: false,
		},
		{
			name: "LongEven",
			input: innerWireFormat{
				HostID:        12345,
				MsgID:         67819,
				MessageSeq:    1,
				MessageSeqMax: 5,
				Timestamp:     uint64(time.Now().Unix()),
				Hostname:      []byte("localhost"),
				ContextFields: []contextWireFormat{
					{
						Key:     []byte("applicationname"),
						valType: ContextString,
						Value:   []byte("user"),
					},
					{
						Key:     []byte("facility"),
						valType: ContextInt64,
						Value:   mockInt,
					},
				},
				Data:       []byte(strings.Repeat("t", 256)),
				PaddingLen: 28,
			},
			expectedErr: false,
		},
		{
			name: "LongOdd",
			input: innerWireFormat{
				HostID:        12345,
				MsgID:         67839,
				MessageSeq:    1,
				MessageSeqMax: 5,
				Timestamp:     uint64(time.Now().Unix()),
				Hostname:      []byte("localhost"),
				ContextFields: []contextWireFormat{
					{
						Key:     []byte("applicationname"),
						valType: ContextString,
						Value:   []byte("user"),
					},
					{
						Key:     []byte("facility"),
						valType: ContextInt64,
						Value:   mockInt,
					},
				},
				Data:       []byte(strings.Repeat("t", 550)),
				PaddingLen: 38,
			},
			expectedErr: false,
		},
		{
			name: "Short",
			input: innerWireFormat{
				HostID:        1,
				MsgID:         1,
				MessageSeq:    1,
				MessageSeqMax: 5,
				Timestamp:     uint64(time.Now().Unix()),
				Hostname:      []byte("a"),
				ContextFields: []contextWireFormat{},
				Data:          []byte("t"),
				PaddingLen:    1,
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
			if deserialized.MsgID != tt.input.MsgID {
				t.Errorf("MsgID: expected %d, got %d", tt.input.MsgID, deserialized.MsgID)
			}
			if deserialized.MessageSeq != tt.input.MessageSeq {
				t.Errorf("MessageSeq: expected %d, got %d", tt.input.MessageSeq, deserialized.MessageSeq)
			}
			if deserialized.MessageSeqMax != tt.input.MessageSeqMax {
				t.Errorf("MessageSeqMax: expected %d, got %d", tt.input.MessageSeqMax, deserialized.MessageSeqMax)
			}
			if deserialized.Timestamp != tt.input.Timestamp {
				t.Errorf("Timestamp: expected %d, got %d", tt.input.Timestamp, deserialized.Timestamp)
			}
			if !bytes.Equal(deserialized.Hostname, tt.input.Hostname) {
				t.Errorf("Hostname: expected %v, got %v", tt.input.Hostname, deserialized.Hostname)
			}
			for index, ctxField := range deserialized.ContextFields {
				if !bytes.Equal(ctxField.Key, tt.input.ContextFields[index].Key) {
					t.Errorf("Context field Key: expected %x, got %x", tt.input.ContextFields[index].Key, ctxField.Key)
				}
				if ctxField.valType != tt.input.ContextFields[index].valType {
					t.Errorf("Context field value type: expected %x, got %x", tt.input.ContextFields[index].valType, ctxField.valType)
				}
				if !bytes.Equal(ctxField.Value, tt.input.ContextFields[index].Value) {
					t.Errorf("Context field value: expected %x, got %x", tt.input.ContextFields[index].Value, ctxField.Value)
				}
			}
			if !bytes.Equal(deserialized.Data, tt.input.Data) {
				t.Errorf("Data: expected %v, got %v", tt.input.Data, deserialized.Data)
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
	fields := innerWireFormat{
		HostID:        12345,
		MsgID:         96789,
		MessageSeq:    1,
		MessageSeqMax: 5,
		Timestamp:     uint64(time.Now().Unix()),
		Hostname:      []byte("localhost"),
		ContextFields: []contextWireFormat{},
		Data:          []byte(strings.Repeat("t", 65536)),
		PaddingLen:    8,
	}
	_, err := ConstructInnerPayload(fields)
	if err == nil {
		t.Error("Expected error for longer payload, but got none")
	}
}

func TestInvaliData(t *testing.T) {
	fields := innerWireFormat{
		HostID:        12345,
		MsgID:         96789,
		MessageSeq:    1,
		MessageSeqMax: 5,
		Timestamp:     uint64(time.Now().Unix()),
		Hostname:      []byte("localhost"),
		ContextFields: []contextWireFormat{
			{
				Key:     []byte("applicationname"),
				valType: ContextString,
				Value:   []byte("user"),
			},
		},
		Data:       []byte(""),
		PaddingLen: 8,
	}

	_, err := ConstructInnerPayload(fields)
	if err == nil {
		t.Fatalf("Expected error for construction, but got nil")
	}
	expectedErr := "failed to serialize Data: field cannot be empty"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got %v", expectedErr, err)
	}
}

func TestPaddingLen(t *testing.T) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, int64(1234))
	if err != nil {
		t.Fatalf("failed to mock integer: %v", err)
	}
	mockInt := buf.Bytes()

	// Test that padding is added correctly
	fields := innerWireFormat{
		HostID:        12345,
		MsgID:         96789,
		MessageSeq:    1,
		MessageSeqMax: 5,
		Timestamp:     uint64(time.Now().Unix()),
		Hostname:      []byte("localhost"),
		ContextFields: []contextWireFormat{
			{
				Key:     []byte("applicationname"),
				valType: ContextString,
				Value:   []byte("user"),
			},
			{
				Key:     []byte("facility"),
				valType: ContextInt64,
				Value:   mockInt,
			},
		},
		Data:       []byte("This is a test log message"),
		PaddingLen: 8,
	}

	// Serialize the payload
	serialized, _ := ConstructInnerPayload(fields)

	// Calculate the expected length based on test case
	expectedLen := 0
	expectedLen += lenHostID                    // HostID (uint32)
	expectedLen += lenMsgID                     // MsgID (uint16)
	expectedLen += lenMsgSeq                    // MessageSeq (uint16)
	expectedLen += lenSeqMax                    // MessageSeqMax (uint16)
	expectedLen += lenTimestamp                 // Timestamp (uint64)
	expectedLen += 1 + len(fields.Hostname) + 1 // Hostname length + Hostname + Null terminator
	expectedLen += lenContextSectionNxtLen      // Context nxt length
	for _, field := range fields.ContextFields {
		expectedLen += lenCtxKeyNxtLen
		expectedLen += len(field.Key)
		expectedLen += lenCtxKeyTerminator
		expectedLen += lenCtxTypeVal
		expectedLen += lenCtxValNxtLen
		expectedLen += len(field.Value)
		expectedLen += lenCtxValTerminator
	}
	expectedLen += lenContextSectionTerminator // Context Null terminator
	expectedLen += 1 + len(fields.Data) + 2    // LogText length + LogText + Null terminator
	expectedLen += fields.PaddingLen           // Padding

	// Check that padding is added
	if len(serialized) != expectedLen {
		t.Errorf("Padding length is incorrect. Expected %d, got %d", expectedLen, len(serialized))
	}
}

func TestInvalidNextLengthByte(t *testing.T) {
	fields := innerWireFormat{
		HostID:        12345,
		MsgID:         96789,
		MessageSeq:    1,
		MessageSeqMax: 5,
		Timestamp:     uint64(time.Now().Unix()),
		Hostname:      []byte("localhost"),
		ContextFields: []contextWireFormat{
			{
				Key:     []byte("applicationname"),
				valType: ContextString,
				Value:   []byte("user"),
			},
		},
		Data:       []byte("This is a test log message"),
		PaddingLen: 8,
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
	}
}
