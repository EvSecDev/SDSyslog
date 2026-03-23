package protocol

import (
	"bytes"
	"crypto/ed25519"
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
		signingPrivKey   []byte
		cryptoID         uint8
		sigID            uint8
		hostID           int
		maxPayloadSize   int
		mutatePackets    func(packets [][]byte) [][]byte
		pinnedPubKeys    map[string][]byte
		expectedMsg      Message
		expectErrCreate  string
		expectErrExtract string
		expectEmptyRecv  bool
	}{
		{
			name: "single fragment - small payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-a",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
			hostID:         1,
			maxPayloadSize: 1024,
			cryptoID:       1,
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "host-a",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
		},
		{
			name: "valid signature",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-a-w-sig",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
			hostID:         1,
			maxPayloadSize: 1024,
			signingPrivKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)),
			cryptoID:       1,
			sigID:          1,
			pinnedPubKeys: map[string][]byte{
				"host-a-w-sig": ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)).Public().(ed25519.PublicKey),
			},
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  "host-a-w-sig",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
		},
		{
			name: "hostname forgery (incorrect signature)",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-a-fake",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
			hostID:         1,
			maxPayloadSize: 1024,
			signingPrivKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)),
			cryptoID:       1,
			sigID:          1,
			pinnedPubKeys: map[string][]byte{
				"host-a-fake": ed25519.NewKeyFromSeed(bytes.Repeat([]byte("y"), ed25519.SeedSize)).Public().(ed25519.PublicKey),
			},
			expectErrExtract: "payload with alleged hostname \"host-a-fake\" has invalid signature",
		},
		{
			name: "hostname forgery (no signature)",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-a-fake-2",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
			hostID:         1,
			maxPayloadSize: 1024,
			signingPrivKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)),
			cryptoID:       1,
			sigID:          0,
			pinnedPubKeys: map[string][]byte{
				"host-a-fake-2": ed25519.NewKeyFromSeed(bytes.Repeat([]byte("y"), ed25519.SeedSize)).Public().(ed25519.PublicKey),
			},
			expectErrExtract: "sender has a pinned key but received packet has no signature",
		},
		{
			name: "unknown hostname with signature",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-unk-w-sig",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
			hostID:         1,
			maxPayloadSize: 1024,
			signingPrivKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)),
			cryptoID:       1,
			sigID:          1,
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnkSig + "host-unk-w-sig",
				Fields:    map[string]any{"env": "dev"},
				Data:      []byte("hello world"),
			},
		},
		{
			name: "multi fragment - large payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-b",
				Fields:    map[string]any{"env": "prod"},
				Data:      bytes.Repeat([]byte("A"), 10_000),
			},
			hostID:         42,
			maxPayloadSize: 256,
			cryptoID:       1,
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "host-b",
				Fields:    map[string]any{"env": "prod"},
				Data:      bytes.Repeat([]byte("A"), 10_000),
			},
		},
		{
			name: "multi fragment - large payload - with signature",
			msg: Message{
				Timestamp: now,
				Hostname:  "host-large-w-sig",
				Fields:    map[string]any{"env": "prod"},
				Data:      bytes.Repeat([]byte("A"), 10_000),
			},
			hostID:         42,
			maxPayloadSize: 256,
			signingPrivKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)),
			cryptoID:       1,
			sigID:          1,
			pinnedPubKeys: map[string][]byte{
				"host-large-w-sig": ed25519.NewKeyFromSeed(bytes.Repeat([]byte("x"), ed25519.SeedSize)).Public().(ed25519.PublicKey),
			},
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  "host-large-w-sig",
				Fields:    map[string]any{"env": "prod"},
				Data:      bytes.Repeat([]byte("A"), 10_000),
			},
		},
		{
			name: "exact boundary payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "boundary-host",
				Data:      bytes.Repeat([]byte("B"), 512),
			},
			hostID:         7,
			maxPayloadSize: 512,
			cryptoID:       1,
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "boundary-host",
				Data:      bytes.Repeat([]byte("B"), 512),
			},
		},
		{
			name: "unicode payload",
			msg: Message{
				Timestamp: now,
				Hostname:  "unicode-host",
				Fields:    map[string]any{"lang": ":)"},
				Data:      []byte("hello there🌏"),
			},
			hostID:         9,
			maxPayloadSize: 256,
			cryptoID:       1,
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "unicode-host",
				Fields:    map[string]any{"lang": ":)"},
				Data:      []byte("hello there🌏"),
			},
		},
		{
			name: "invalid max payload size",
			msg: Message{
				Timestamp: now,
				Hostname:  "bad-payload",
				Data:      []byte("test"),
			},
			hostID:          1,
			maxPayloadSize:  0,
			cryptoID:        1,
			expectErrCreate: "maxPayloadSize must be greater than 0",
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "bad-payload",
				Data:      []byte("test"),
			},
		},
		{
			name: "custom fields exceed max payload size",
			msg: Message{
				Timestamp: now,
				Hostname:  "too-bid-fields",
				Fields: map[string]any{
					"field1": strings.Repeat("a", 255),
				},
				Data: []byte("test"),
			},
			hostID:          1,
			maxPayloadSize:  260,
			cryptoID:        1,
			expectErrCreate: "exceeded max payload size: no room left for message in packet",
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "too-bid-fields",
				Fields: map[string]any{
					"field1": strings.Repeat("a", 255),
				},
				Data: []byte("test"),
			},
		},
		{
			name: "corrupted packet",
			msg: Message{
				Timestamp: now,
				Hostname:  "corrupt-host",
				Data:      []byte("valid data"),
			},
			hostID:         5,
			maxPayloadSize: 256,
			cryptoID:       1,
			mutatePackets: func(packets [][]byte) [][]byte {
				packets[0][0] ^= 0xFF
				return packets
			},
			expectErrExtract: "failed to deserialize outer payload for fragment 0: " + ErrUnknownSuite.Error() + ": ID 254",
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "corrupt-host",
				Data:      []byte("valid data"),
			},
		},
		{
			name: "reordered fragments",
			msg: Message{
				Timestamp: now,
				Hostname:  "reorder-host",
				Data:      bytes.Repeat([]byte("Z"), 2048),
			},
			hostID:         12,
			maxPayloadSize: 256,
			cryptoID:       1,
			mutatePackets: func(packets [][]byte) [][]byte {
				if len(packets) > 1 {
					packets[0], packets[1] = packets[1], packets[0]
				}
				return packets
			},
			expectedMsg: Message{
				Timestamp: now,
				Hostname:  HostPrefixUnverified + "reorder-host",
				Data:      bytes.Repeat([]byte("Z"), 2048),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup create signature function
			if len(tt.signingPrivKey) > 0 {
				err = wrappers.SetupCreateSignature(tt.signingPrivKey)
				if err != nil {
					t.Fatalf("expected no error creating signing function, but got: %v", err)
				}
			}

			packets, err := Create(tt.msg, tt.hostID, tt.maxPayloadSize, tt.cryptoID, tt.sigID)
			if tt.expectErrCreate != "" {
				if err == nil {
					t.Fatalf("expected create error, got nil")
				}
				if !strings.Contains(err.Error(), tt.expectErrCreate) {
					t.Fatalf("expected error %q, but got %q", tt.expectErrCreate, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected create error: %v", err)
			}

			if len(tt.pinnedPubKeys) > 0 {
				// Init signature verification function
				err = wrappers.SetupVerifySignature(tt.pinnedPubKeys)
				if err != nil {
					t.Fatalf("expected no error creating verification function, but got: %v", err)
				}
			}

			if tt.expectEmptyRecv {
				recvMsg, recvHostID, err := Extract(nil)
				if err != nil {
					t.Fatalf("unexpected extract error: %v", err)
				}
				if len(recvMsg.Data) == 0 {
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

			// Verify packet sizes
			for _, packet := range packets {
				if len(packet) > tt.maxPayloadSize {
					t.Fatalf("expected maximum payload size to create packets of size %d, but got packet of size %d",
						tt.maxPayloadSize, len(packet))
				}
			}

			// Optional packet mutation
			if tt.mutatePackets != nil {
				packets = tt.mutatePackets(packets)
			}

			recvMsg, recvHostID, err := Extract(packets)
			if tt.expectErrExtract != "" {
				if err == nil {
					t.Fatalf("expected extract error, got nil")
				}
				if !strings.Contains(err.Error(), tt.expectErrExtract) {
					t.Fatalf("expected error %q, but got %q", tt.expectErrExtract, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected extract error: %v", err)
			}

			// Limited compare due to timing. Assert down to day only
			if recvMsg.Timestamp.Year() != tt.expectedMsg.Timestamp.Year() {
				t.Fatalf("timestamp mismatch: got %v want %v", recvMsg.Timestamp, tt.expectedMsg.Timestamp)
			} else if recvMsg.Timestamp.Month() != tt.expectedMsg.Timestamp.Month() {
				t.Fatalf("timestamp mismatch: got %v want %v", recvMsg.Timestamp, tt.expectedMsg.Timestamp)
			} else if recvMsg.Timestamp.Day() != tt.expectedMsg.Timestamp.Day() {
				t.Fatalf("timestamp mismatch: got %v want %v", recvMsg.Timestamp, tt.expectedMsg.Timestamp)
			}
			if recvMsg.Hostname != tt.expectedMsg.Hostname {
				t.Fatalf("hostname mismatch: got %q want %q", recvMsg.Hostname, tt.expectedMsg.Hostname)
			}
			if len(recvMsg.Fields) != len(tt.expectedMsg.Fields) {
				t.Fatalf("fields length mismatch: got %d want %d", len(recvMsg.Fields), len(tt.expectedMsg.Fields))
			}
			for k, v := range tt.expectedMsg.Fields {
				if recvMsg.Fields[k] != v {
					t.Fatalf("field %q mismatch: got %q want %q", k, recvMsg.Fields[k], v)
				}
			}
			if !bytes.Equal(recvMsg.Data, tt.expectedMsg.Data) {
				t.Fatalf("data mismatch: got %q want %q", recvMsg.Data, tt.expectedMsg.Data)
			}
			if recvHostID != tt.hostID {
				t.Fatalf("hostID mismatch: got %d want %d", recvHostID, tt.hostID)
			}
		})
	}
}
