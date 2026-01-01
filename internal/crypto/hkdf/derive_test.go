package hkdf

import (
	"bytes"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	tests := []struct {
		name      string
		secret    []byte
		salt      []byte
		namespace string
		keySize   int
		expectErr bool
	}{
		{
			name:      "Basic 32-byte key",
			secret:    []byte("secret"),
			salt:      []byte("salt"),
			namespace: "example",
			keySize:   32,
			expectErr: false,
		},
		{
			name:      "Different salt changes output",
			secret:    []byte("secret"),
			salt:      []byte("other_salt"),
			namespace: "example",
			keySize:   32,
			expectErr: false,
		},
		{
			name:      "Different secret changes output",
			secret:    []byte("different_secret"),
			salt:      []byte("salt"),
			namespace: "example",
			keySize:   32,
			expectErr: false,
		},
		{
			name:      "Different namespace changes output",
			secret:    []byte("secret"),
			salt:      []byte("salt"),
			namespace: "other_namespace",
			keySize:   32,
			expectErr: false,
		},
		{
			name:      "Larger key size 64 bytes",
			secret:    []byte("secret"),
			salt:      []byte("salt"),
			namespace: "example",
			keySize:   64,
			expectErr: false,
		},
		{
			name:      "Zero-length secret and salt",
			secret:    []byte{},
			salt:      []byte{},
			namespace: "empty",
			keySize:   32,
			expectErr: false,
		},
		{
			name:      "Nil secret and salt",
			secret:    nil,
			salt:      nil,
			namespace: "nil_case",
			keySize:   32,
			expectErr: false,
		},
		{
			name:      "Very short key (1 byte)",
			secret:    []byte("secret"),
			salt:      []byte("salt"),
			namespace: "short_key",
			keySize:   1,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of secret and salt (not testing memory zero)
			keyCopy := make([]byte, len(tt.secret))
			copy(keyCopy, tt.secret)
			saltCopy := make([]byte, len(tt.salt))
			copy(saltCopy, tt.salt)

			newKey, err := DeriveKey(tt.secret, tt.salt, tt.namespace, tt.keySize)
			if err != nil && !tt.expectErr {
				t.Errorf("expected no error, but got error '%v'", err)
			}
			if err == nil && tt.expectErr {
				t.Error("expected error, but got no error")
			}

			// Verify correct key length
			if len(newKey) != tt.keySize {
				t.Fatalf("expected key size %d, got %d", tt.keySize, len(newKey))
			}

			// Deterministic: same inputs produce same result
			again, err := DeriveKey(keyCopy, saltCopy, tt.namespace, tt.keySize)
			if err != nil && !tt.expectErr {
				t.Errorf("expected no error, but got error '%v'", err)
			}
			if err == nil && tt.expectErr {
				t.Error("expected error, but got no error")
			}
			if !bytes.Equal(newKey, again) {
				t.Error("expected deterministic output, but keys differ")
			}

			// Uniqueness: ensure different variations produce distinct outputs
			// (basic check for non-empty inputs)
			if len(keyCopy) > 0 && len(saltCopy) > 0 {
				base, err := DeriveKey([]byte("base_secret"), []byte("base_salt"), "base_ns", tt.keySize)
				if err != nil && !tt.expectErr {
					t.Errorf("expected no error, but got error '%v'", err)
				}
				if err == nil && tt.expectErr {
					t.Error("expected error, but got no error")
				}
				if bytes.Equal(newKey, base) {
					t.Error("unexpected identical output with different inputs")
				}
			}

			// Ensure no all-zero output
			zero := make([]byte, tt.keySize)
			if bytes.Equal(newKey, zero) {
				t.Error("unexpected all-zero derived key")
			}
		})
	}
}
