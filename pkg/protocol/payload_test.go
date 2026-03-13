package protocol

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/crypto/wrappers"
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
		sigID    uint8
		expected innerWireFormat
		err      string
	}{
		{
			name: "valid payload no sig",
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
			sigID: 0,
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
						Value:   []byte(EmptyFieldChar),
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
			name: "valid payload create signature",
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
			sigID: 1,
			expected: innerWireFormat{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Hostname:      []byte("test-host"),
				SignatureID:   1,
				ContextFields: []contextWireFormat{
					{
						Key:     []byte("applicationname"),
						valType: ContextString,
						Value:   []byte("app1"),
					},
					{
						Key:     []byte("marker"),
						valType: ContextString,
						Value:   []byte(EmptyFieldChar),
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
			name: "invalid precomputed signature",
			input: Payload{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Hostname:      "test-host",
				SignatureID:   1,
				Signature:     bytes.Repeat([]byte("x"), ed25519.SignatureSize-1),
				CustomFields: map[string]any{
					"applicationname": "app1",
				},
				Data:       []byte("log message"),
				PaddingLen: 18,
			},
			sigID: 1,
			err:   "signature length 63 for id 1 must be between 64 and 64 bytes",
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
			// Initialize signing functions for this test (also relies on setup funcs being callable multiple times)
			var randSource []byte
			err = random.PopulateEmptySlice(&randSource, ed25519.SeedSize)
			if err != nil {
				t.Fatalf("expected no error creating ed25519 signing key, but got: %v", err)
			}
			priv := ed25519.NewKeyFromSeed(randSource)
			err = wrappers.SetupCreateSignature(priv)
			if err != nil {
				t.Fatalf("expected no error creating signing function, but got: %v", err)
			}
			publicKey := priv.Public().(ed25519.PublicKey)
			pinnedKeys := map[string][]byte{
				tt.input.Hostname: publicKey,
			}
			err = wrappers.SetupVerifySignature(pinnedKeys)
			if err != nil {
				t.Fatalf("expected no error creating verification function, but got: %v", err)
			}

			ttTime := time.Now()
			tt.input.Timestamp = ttTime
			tt.expected.Timestamp = uint64(ttTime.UnixMilli())

			proto, err := ConstructPayload(tt.input, tt.sigID)
			if tt.err != "" {
				if err == nil {
					t.Fatalf("expected error but got nil")
				} else if err.Error() != tt.err {
					t.Fatalf("expected error '%v' but got '%v'", tt.err, err)
				}
				return
			} else {
				if err != nil {
					t.Fatalf("unexpected error: '%v'", err)
				}
				if !compareProtocols(proto, tt.expected) {
					t.Fatalf("Protocols not equal:\n Expected '%v'\n Got      '%v'", tt.expected, proto)
				}
			}

			if tt.sigID > 0 {
				// Validate signature
				signedBytes := proto.SerializeForSignature()
				valid, err := wrappers.VerifySignature(publicKey, signedBytes, proto.Signature, tt.sigID)
				if err != nil {
					t.Errorf("unexpected error verifying signature: '%v'", err)
				}
				if !valid {
					t.Fatalf("expected verification post signing to return valid signature, but got false")
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
			name: "valid protocol no signature",
			input: innerWireFormat{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Timestamp:     uint64(time.Now().UnixMilli()),
				Hostname:      []byte("test-host"),
				SignatureID:   0,
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
				Hostname:      HostPrefixUnverified + "test-host",
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
			name: "valid protocol with signature",
			input: innerWireFormat{
				HostID:        1,
				MsgID:         2,
				MessageSeq:    3,
				MessageSeqMax: 4,
				Timestamp:     uint64(time.Now().UnixMilli()),
				Hostname:      []byte("test-host"),
				SignatureID:   1,
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
				SignatureID:   1,
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
			// Initialize signing functions for this test (also relies on setup funcs being callable multiple times)
			var randSource []byte
			err = random.PopulateEmptySlice(&randSource, ed25519.SeedSize)
			if err != nil {
				t.Fatalf("expected no error creating ed25519 signing key, but got: %v", err)
			}
			priv := ed25519.NewKeyFromSeed(randSource)
			err = wrappers.SetupCreateSignature(priv)
			if err != nil {
				t.Fatalf("expected no error creating signing function, but got: %v", err)
			}
			publicKey := priv.Public().(ed25519.PublicKey)

			var pinnedKeys map[string][]byte
			if tt.input.SignatureID > 0 {
				pinnedKeys = map[string][]byte{
					string(tt.input.Hostname): publicKey,
				}
			} else {
				pinnedKeys = make(map[string][]byte)
			}

			err = wrappers.SetupVerifySignature(pinnedKeys)
			if err != nil {
				t.Fatalf("expected no error creating verification function, but got: %v", err)
			}

			bytesToSign := tt.input.SerializeForSignature()
			tt.input.Signature, err = wrappers.CreateSignature(bytesToSign, tt.input.SignatureID)
			if err != nil {
				t.Fatalf("expected no error creating mock signature, but got: %v", err)
			}

			request, err := DeconstructPayload(tt.input)
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

			if tt.input.SignatureID > 0 {
				// Validate signature pass-through
				if !bytes.Equal(tt.input.Signature, request.Signature) {
					t.Errorf("expected signature field to be present pre and post deconstruction, but fields differ")
					t.Errorf("  expected signature: %x", tt.input.Signature)
					t.Errorf("  got signature     : %x", request.Signature)
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
		p1.SignatureID == p2.SignatureID &&
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
