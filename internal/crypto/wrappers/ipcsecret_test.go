package wrappers

import (
	"bytes"
	"sdsyslog/internal/crypto/ecdh"
	"testing"
)

func TestSetupGetSharedSecret(t *testing.T) {
	tests := []struct {
		name         string
		privKey      []byte
		expectErr    bool
		expectSecret bool
	}{
		{
			name:         "valid key",
			privKey:      make([]byte, 32),
			expectErr:    false,
			expectSecret: true,
		},
		{
			name:         "valid long key",
			privKey:      make([]byte, 128),
			expectErr:    false,
			expectSecret: true,
		},
		{
			name:         "empty key with no previous getter",
			privKey:      nil,
			expectErr:    true,
			expectSecret: false,
		},
		{
			name:         "empty key with previous getter",
			privKey:      nil,
			expectErr:    false,
			expectSecret: true,
		},
	}

	// Mock a valid persistent key for tests
	priv, _, err := ecdh.CreatePersistentKey()
	if err != nil {
		t.Fatalf("failed to create mock persistent key: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the mock key if test key is zero length
			key := tt.privKey
			if key == nil && tt.expectSecret {
				key = priv
			}

			// Reset global before each test
			GetSharedSecret = nil

			err := SetupGetSharedSecret(key)
			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error=%v, got %v", tt.expectErr, err)
			}

			if tt.expectSecret {
				if GetSharedSecret == nil {
					t.Fatalf("expected GetSharedSecret to be initialized")
				}

				secret1 := GetSharedSecret()
				secret2 := GetSharedSecret()

				if !bytes.Equal(secret1, secret2) {
					t.Fatalf("expected repeated calls to return same secret bytes")
				}

				secret1[0] ^= 0xFF
				secret3 := GetSharedSecret()
				if secret1[0] == secret3[0] {
					t.Fatalf("modifying returned secret should not mutate internal secret")
				}
			} else {
				if GetSharedSecret != nil {
					t.Fatalf("expected GetSharedSecret to be nil")
				}
			}
		})
	}
}
