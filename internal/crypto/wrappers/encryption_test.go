package wrappers

import (
	"bytes"
	"sdsyslog/internal/crypto/ecdh"
	"strings"
	"testing"
)

func TestEncryptionWrapper(t *testing.T) {
	priv, pub, err := ecdh.CreatePersistentKey()
	if err != nil {
		panic(err)
	}

	suiteID := uint8(1)

	tests := []struct {
		name              string
		privateKey        []byte
		publicKey         []byte
		payload           []byte
		expectedPayload   []byte
		expectedEncError  bool
		expectedDecrError bool
	}{
		{
			name:              "regular",
			privateKey:        priv,
			publicKey:         pub,
			payload:           []byte("some testing text"),
			expectedPayload:   []byte("some testing text"),
			expectedEncError:  false,
			expectedDecrError: false,
		},
		{
			name:              "large",
			privateKey:        priv,
			publicKey:         pub,
			payload:           []byte(strings.Repeat("t", 1000)),
			expectedPayload:   []byte(strings.Repeat("t", 1000)),
			expectedEncError:  false,
			expectedDecrError: false,
		},
		{
			name:              "nil",
			privateKey:        priv,
			publicKey:         pub,
			payload:           nil,
			expectedPayload:   nil,
			expectedEncError:  false,
			expectedDecrError: false,
		},
		{
			name:              "nil public",
			privateKey:        priv,
			publicKey:         nil,
			payload:           []byte("some testing text"),
			expectedPayload:   []byte("some testing text"),
			expectedEncError:  true,
			expectedDecrError: false,
		},
		{
			name:              "nil private",
			privateKey:        nil,
			publicKey:         pub,
			payload:           []byte("some testing text"),
			expectedPayload:   []byte("some testing text"),
			expectedEncError:  false,
			expectedDecrError: true,
		},
		{
			name:              "empty payload",
			privateKey:        priv,
			publicKey:         pub,
			payload:           []byte{},
			expectedPayload:   []byte{},
			expectedEncError:  false,
			expectedDecrError: false,
		},
		{
			name:              "corrupted ciphertext",
			privateKey:        priv,
			publicKey:         pub,
			payload:           []byte("some testing text"),
			expectedPayload:   nil,
			expectedEncError:  false,
			expectedDecrError: true, // Corrupt data will cause decryption failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetupDecryptInnerPayload(tt.privateKey)
			SetupEncryptInnerPayload(tt.publicKey)

			ciphertext, ephemeralPub, nonce, err := EncryptInnerPayload(tt.payload, suiteID)
			if err != nil && !tt.expectedEncError {
				t.Errorf("expected no encryption error, but got '%v'", err)
				return
			}
			if err == nil && tt.expectedEncError {
				t.Errorf("expected encryption error, but got no error")
			}
			if tt.expectedEncError {
				return // encryption failures will fail decryptions
			}

			// Simulate corruption or error before decryption
			if tt.name == "corrupted ciphertext" {
				ciphertext[0] ^= 0x01 // change a byte
			}

			outPayload, err := DecryptInnerPayload(ciphertext, ephemeralPub, nonce, suiteID)
			if err != nil && !tt.expectedDecrError {
				t.Errorf("expected no decryption error, but got '%v'", err)
				return
			}
			if err == nil && tt.expectedDecrError {
				t.Errorf("expected decryption error, but got no error")
			}
			if tt.expectedDecrError {
				return // decryption failures will fail content compares
			}

			if !bytes.Equal(outPayload, tt.expectedPayload) {
				t.Errorf("decrypted output payload does not match expected payload:\n  Expected Payload: %v\n  Actual Payload: %v\n", tt.expectedPayload, outPayload)
			}
		})
	}
}
