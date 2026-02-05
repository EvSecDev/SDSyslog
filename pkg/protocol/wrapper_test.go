package protocol

import (
	"bytes"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/wrappers"
	"strings"
	"testing"
	"time"
)

func TestProtocol(t *testing.T) {
	private, public, err := ecdh.CreatePersistentKey()
	if err != nil {
		t.Fatalf("failed to generate test keys: %v", err)
		return
	}
	err = wrappers.SetupDecryptInnerPayload(private)
	if err != nil {
		t.Fatalf("failed to setup decryption function: %v", err)
		return
	}
	err = wrappers.SetupEncryptInnerPayload(public)
	if err != nil {
		t.Fatalf("failed to setup decryption function: %v", err)
		return
	}

	now := time.Now()

	tests := []struct {
		name             string
		msg              Message
		hostID           int
		maxPayloadSize   int
		mutatePackets    func(packets [][]byte) [][]byte
		expectErrCreate  bool
		expectErrExtract bool
		expectEmptyRecv  bool
	}{
		{
			name: "single fragment - small payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-a",
				Fields:    map[string]any{"env": "dev"},
				Data:      "hello world",
			},
			hostID:         1,
			maxPayloadSize: 1024,
		},
		{
			name: "multi fragment - large payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-b",
				Fields:    map[string]any{"env": "prod"},
				Data:      string(bytes.Repeat([]byte("A"), 10_000)),
			},
			hostID:         42,
			maxPayloadSize: 256,
		},
		{
			name: "exact boundary payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "boundary-host",
				Data:      string(bytes.Repeat([]byte("B"), 512)),
			},
			hostID:         7,
			maxPayloadSize: 512,
		},
		{
			name: "unicode payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "unicode-host",
				Fields:    map[string]any{"lang": ":)"},
				Data:      "hello there🌏",
			},
			hostID:         9,
			maxPayloadSize: 256,
		},
		{
			name: "invalid max payload size",
			msg: Message{
				Timestamp: now,
				Hostname:  "bad-payload",
				Data:      "test",
			},
			hostID:          1,
			maxPayloadSize:  0,
			expectErrCreate: true,
		},
		{
			name: "custom fields exceed max payload size",
			msg: Message{
				Timestamp: now,
				Hostname:  "too-bid-fields",
				Fields: map[string]any{
					"field1": strings.Repeat("a", 255),
				},
				Data: "test",
			},
			hostID:          1,
			maxPayloadSize:  260,
			expectErrCreate: true,
		},
		{
			name: "corrupted packet",
			msg: Message{
				Timestamp: now,
				Hostname:  "corrupt-host",
				Data:      "valid data",
			},
			hostID:         5,
			maxPayloadSize: 256,
			mutatePackets: func(packets [][]byte) [][]byte {
				packets[0][0] ^= 0xFF
				return packets
			},
			expectErrExtract: true,
		},
		{
			name: "reordered fragments",
			msg: Message{
				Timestamp: now,
				Hostname:  "reorder-host",
				Data:      string(bytes.Repeat([]byte("Z"), 2048)),
			},
			hostID:         12,
			maxPayloadSize: 256,
			mutatePackets: func(packets [][]byte) [][]byte {
				if len(packets) > 1 {
					packets[0], packets[1] = packets[1], packets[0]
				}
				return packets
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packets, err := Create(tt.msg, tt.hostID, tt.maxPayloadSize)
			if tt.expectErrCreate {
				if err == nil {
					t.Fatalf("expected create error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected create error: %v", err)
			}

			if tt.expectEmptyRecv {
				recvMsg, recvHostID, err := Extract(nil)
				if err != nil {
					t.Fatalf("unexpected extract error: %v", err)
				}
				if recvMsg.Data != "" {
					t.Fatalf("expected empty message, got %+v", recvMsg)
				}
				if recvHostID != 0 {
					t.Fatalf("expected hostID 0, got %d", recvHostID)
				}
				return
			}

			if len(packets) == 0 {
				t.Fatalf("expected packets, got none")
			}

			// Optional packet mutation
			if tt.mutatePackets != nil {
				packets = tt.mutatePackets(packets)
			}

			recvMsg, recvHostID, err := Extract(packets)
			if tt.expectErrExtract {
				if err == nil {
					t.Fatalf("expected extract error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected extract error: %v", err)
			}

			// Limited compare due to timing. Assert down to day only
			if recvMsg.Timestamp.Year() != tt.msg.Timestamp.Year() {
				t.Fatalf("timestamp mismatch: got %v want %v", recvMsg.Timestamp, tt.msg.Timestamp)
			} else if recvMsg.Timestamp.Month() != tt.msg.Timestamp.Month() {
				t.Fatalf("timestamp mismatch: got %v want %v", recvMsg.Timestamp, tt.msg.Timestamp)
			} else if recvMsg.Timestamp.Day() != tt.msg.Timestamp.Day() {
				t.Fatalf("timestamp mismatch: got %v want %v", recvMsg.Timestamp, tt.msg.Timestamp)
			}
			if recvMsg.Hostname != tt.msg.Hostname {
				t.Fatalf("hostname mismatch: got %q want %q", recvMsg.Hostname, tt.msg.Hostname)
			}
			if len(recvMsg.Fields) != len(tt.msg.Fields) {
				t.Fatalf("fields length mismatch: got %d want %d", len(recvMsg.Fields), len(tt.msg.Fields))
			}
			for k, v := range tt.msg.Fields {
				if recvMsg.Fields[k] != v {
					t.Fatalf("field %q mismatch: got %q want %q", k, recvMsg.Fields[k], v)
				}
			}
			if recvMsg.Data != tt.msg.Data {
				t.Fatalf("data mismatch: got %q want %q", recvMsg.Data, tt.msg.Data)
			}
			if recvHostID != tt.hostID {
				t.Fatalf("hostID mismatch: got %d want %d", recvHostID, tt.hostID)
			}
		})
	}
}
