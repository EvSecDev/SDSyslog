package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
	"time"
)

func TestConstructPayload(t *testing.T) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, int64(1234))
	if err != nil {
		t.Fatalf("failed to mock integer: %v", err)
	}
	mockInt := buf.Bytes()

	testCases := []struct {
		name     string
		input    Payload
		expected innerWireFormat
		err      string
	}{
		{
			name: "valid payload",
			input: Payload{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Hostname:      "test-host",
				CustomFields: map[string]any{
					"applicationname": "app1",
					"processid":       int64(1234),
					"marker":          nil,
				},
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			expected: innerWireFormat{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Hostname:      []byte("test-host"),
				ContextFields: []contextWireFormat{
					{
						Key:     []byte("applicationname"),
						valType: ContextString,
						Value:   []byte("app1"),
					},
					{
						Key:     []byte("marker"),
						valType: ContextString,
						Value:   []byte(emptyFieldChar),
					},
					{
						Key:     []byte("processid"),
						valType: ContextInt64,
						Value:   mockInt,
					},
				},
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			err: "",
		},
		{
			name: "messageSeq larger than messageSeqMax",
			input: Payload{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    5,
				MessageSeqMax: 4, // Invalid case
				Hostname:      "test-host",
				CustomFields: map[string]any{
					"applicationname": "app1",
					"processid":       1234,
				},
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			expected: innerWireFormat{},
			err:      "message sequence cannot be larger than maximum sequence",
		},
		{
			name: "invalid padding length",
			input: Payload{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Hostname:      "test-host",
				CustomFields: map[string]any{
					"applicationname": "app1",
					"processid":       1234,
				},
				Data:       []byte("log message"),
				PaddingLen: 500, // Invalid padding length
			},
			expected: innerWireFormat{},
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
					t.Errorf("Protocols not equal:\n Expected '%v'\n Got      '%v'", tt.expected, proto)
				}
			}
		})
	}
}

func TestDeconstructPayload(t *testing.T) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, int64(1234))
	if err != nil {
		t.Fatalf("failed to mock integer: %v", err)
	}
	mockInt := buf.Bytes()

	testCases := []struct {
		name     string
		input    innerWireFormat
		expected Payload
		err      string
	}{
		{
			name: "valid protocol",
			input: innerWireFormat{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Timestamp:     uint64(time.Now().UnixMilli()),
				Hostname:      []byte("test-host"),
				ContextFields: []contextWireFormat{
					{
						Key:     []byte("applicationname"),
						valType: ContextString,
						Value:   []byte("app1"),
					},
					{
						Key:     []byte("processid"),
						valType: ContextInt64,
						Value:   mockInt,
					},
				},
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			expected: Payload{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Timestamp:     time.UnixMilli(int64(time.Now().UnixMilli())),
				Hostname:      "test-host",
				CustomFields: map[string]any{
					"applicationname": "app1",
					"processid":       int64(1234),
				},
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			err: "",
		},
		{
			name: "empty host ID",
			input: innerWireFormat{
				HostID:        0,
				MsgID:         1,
				MessageSeq:    1,
				MessageSeqMax: 1,
				Timestamp:     1000,
				Hostname:      []byte("test-host"),
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
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			expected: Payload{},
			err:      "empty host ID",
		},
		{
			name: "empty msg ID",
			input: innerWireFormat{
				HostID:        1,
				MsgID:         0,
				MessageSeq:    1,
				MessageSeqMax: 1,
				Timestamp:     1000,
				Hostname:      []byte("test-host"),
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
				Data:       []byte("log message"),
				PaddingLen: 16,
			},
			expected: Payload{},
			err:      "empty msg ID",
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
					t.Errorf("Payloads not equal:\n Expected '%v'\n Got      '%v'", tt.expected, request)
				}
			}
		})
	}
}

// Helper function to compare Protocol values
func compareProtocols(p1, p2 innerWireFormat) bool {
	basicMatch := p1.HostID == p2.HostID &&
		p1.MsgID == p2.MsgID &&
		p1.MessageSeq == p2.MessageSeq &&
		p1.MessageSeqMax == p2.MessageSeqMax &&
		p1.Timestamp == p2.Timestamp &&
		string(p1.Hostname) == string(p2.Hostname) &&
		string(p1.Data) == string(p2.Data) &&
		p1.PaddingLen == p2.PaddingLen
	if !basicMatch {
		return false
	}
	for index, ctxField := range p1.ContextFields {
		if !bytes.Equal(ctxField.Key, p2.ContextFields[index].Key) {
			return false
		}
		if ctxField.valType != p2.ContextFields[index].valType {
			return false
		}
		if !bytes.Equal(ctxField.Value, p2.ContextFields[index].Value) {
			return false
		}
	}
	return true
}

// Helper function to compare Payload values
func comparePayloads(p1, p2 Payload) bool {
	basicMatch := p1.HostID == p2.HostID &&
		p1.MsgID == p2.MsgID &&
		p1.MessageSeq == p2.MessageSeq &&
		p1.MessageSeqMax == p2.MessageSeqMax &&
		p1.Timestamp.Equal(p2.Timestamp) &&
		p1.Hostname == p2.Hostname &&
		bytes.Equal(p1.Data, p2.Data) &&
		p1.PaddingLen == p2.PaddingLen
	if !basicMatch {
		return false
	}
	for p1Key, p1Value := range p1.CustomFields {
		p2Value, p1KeyPresentinP2 := p2.CustomFields[p1Key]
		if !p1KeyPresentinP2 {
			return false
		}
		if p1Value != p2Value {
			return false
		}
	}
	return true
}
