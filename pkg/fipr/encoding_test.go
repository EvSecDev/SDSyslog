package fipr

import (
	"bytes"
	"encoding/binary"
	"sdsyslog/internal/tests/utils"
	"strings"
	"testing"
)

func TestEncodeDecodeSeq(t *testing.T) {
	tests := []struct {
		name string
		seq  uint16
	}{
		{"Zero", 0},
		{"Small", 42},
		{"Max", maxSequence},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeSeq(tt.seq)
			decoded := decodeSeq(encoded)
			if decoded != tt.seq {
				t.Errorf("decoded %d, want %d", decoded, tt.seq)
			}
		})
	}
}
func TestEncodeDecode(t *testing.T) {
	mockHmacSecret := bytes.Repeat([]byte("x"), HMACSize)
	tests := []struct {
		name            string
		op              opCode
		encodeSession   *Session
		decodeSession   *Session
		payload         []byte
		corruptFunc     func(payload []byte) (corruptedPayload []byte)
		expectEncodeErr error
		expectDecodeErr error
	}{
		{
			name:    "EmptyPayload",
			op:      opMsgCheck,
			payload: nil,
		},
		{
			name:    "NonEmptyPayload",
			op:      opOBO,
			payload: []byte("hello"),
		},
		{
			name: "Encoding on closed session",
			op:   opAck,
			encodeSession: &Session{
				hmacSecret: mockHmacSecret,
				sentFrames: make(map[uint16]framebody),
				seq:        8,
				state:      stateClosed,
			},
			payload:         []byte(encodeSeq(8)),
			expectEncodeErr: ErrSessionClosed,
		},
		{
			name: "Encoding at max sequence",
			op:   opAck,
			encodeSession: &Session{
				hmacSecret: mockHmacSecret,
				sentFrames: make(map[uint16]framebody),
				seq:        maxSequence,
				state:      stateStarted,
			},
			payload:         []byte(encodeSeq(8)),
			expectEncodeErr: ErrBadSequence,
		},
		{
			name: "Decoding at wrong sequence",
			op:   opAck,
			decodeSession: &Session{
				hmacSecret: mockHmacSecret,
				sentFrames: make(map[uint16]framebody),
				seq:        1,
				state:      stateStarted,
			},
			payload:         nil,
			expectDecodeErr: ErrBadSequence,
		},
		{
			name: "Decoding with wrong hamc",
			op:   opStart,
			decodeSession: &Session{
				hmacSecret: bytes.Repeat([]byte("a"), HMACSize),
				sentFrames: make(map[uint16]framebody),
				state:      stateStarted,
			},
			payload:         nil,
			expectDecodeErr: ErrInvalidHMAC,
		},
		{
			name:            "Fail decoding validation with invalid opcode",
			op:              0x94,
			payload:         nil,
			expectDecodeErr: ErrInvalidOpcode,
		},
		{
			name:    "Corrupted frame on wire (truncation)",
			op:      opStart,
			payload: nil,
			corruptFunc: func(payload []byte) (corruptedPayload []byte) {
				corruptedPayload = payload[:minDataLength-minDataLength/2]
				return
			},
			expectDecodeErr: ErrFrameTooShort,
		},
		{
			name:            "Frame larger than maximum",
			op:              opFrgRoute,
			payload:         bytes.Repeat([]byte("x"), 65536),
			expectDecodeErr: ErrFrameTooLarge,
		},
		{
			name:    "Corrupted frame len field",
			op:      opStart,
			payload: nil,
			corruptFunc: func(payload []byte) (corruptedPayload []byte) {
				corruptedPayload = payload
				lenField := int(binary.BigEndian.Uint32(payload[:lenFieldFrameLen]))
				newLen := lenField - lenField/2
				binary.BigEndian.PutUint32(corruptedPayload[0:lenFieldFrameLen], uint32(newLen))
				return
			},
			expectDecodeErr: ErrInvalidFrameLen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.encodeSession == nil {
				tt.encodeSession = &Session{
					hmacSecret: mockHmacSecret,
					sentFrames: make(map[uint16]framebody),
					seq:        0,
				}
			}
			if tt.decodeSession == nil {
				tt.decodeSession = &Session{
					hmacSecret: mockHmacSecret,
					sentFrames: make(map[uint16]framebody),
					seq:        0,
				}
			}

			// Validate encode to bytes
			wireFrame, encodeErr := tt.encodeSession.encodeFrame(tt.op, tt.payload)
			gotExpected, err := utils.MatchWrappedError(encodeErr, tt.expectEncodeErr)
			if err != nil {
				// Ignore encoding validation errors when test is expecting decoding errors
				if !strings.HasPrefix(encodeErr.Error(), "validation:") {
					t.Fatalf("encode: %v", err)
				}
			} else if gotExpected {
				return
			}

			// Validate client recorded its sent frame
			recordedFrame, ok := tt.encodeSession.sentFrames[0]
			if !ok {
				t.Fatal("frame not recorded in client sentFrames")
			}
			if recordedFrame.op != tt.op || !bytes.Equal(recordedFrame.payload, tt.payload) {
				t.Errorf("recorded frame mismatch: got %v, want op %v payload %v", recordedFrame, tt.op, tt.payload)
			}

			// Test malformed frames by corrupting according to test case
			if tt.corruptFunc != nil {
				wireFrame = tt.corruptFunc(wireFrame)
			}

			// Validate decode from bytes
			frame, err := tt.decodeSession.decodeFrame(wireFrame)
			gotExpected, err = utils.MatchWrappedError(err, tt.expectDecodeErr)
			if err != nil {
				t.Fatalf("decode: %v", err)
			} else if gotExpected {
				return
			}

			// Verify integrity
			if frame.op != tt.op {
				t.Errorf("decoded op %v, want %v", frame.op, tt.op)
			}
			if !bytes.Equal(frame.payload, tt.payload) {
				t.Errorf("decoded payload %v, want %v", frame.payload, tt.payload)
			}
			if tt.encodeSession.seq != tt.decodeSession.seq {
				t.Fatalf("expected sessions to have equal sequences, but got: encode seq %d - decode seq %d", tt.encodeSession.seq, tt.decodeSession.seq)
			}
		})
	}
}
