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
		name                   string
		privateKey             []byte
		publicKey              []byte
		payload                []byte
		expectedPayload        []byte
		expectedSetupEncError  bool
		expectedSetupDecrError bool
		expectedEncError       bool
		expectedDecrError      bool
	}{
		{
			name:                   "regular",
			privateKey:             priv,
			publicKey:              pub,
			payload:                []byte("some testing text"),
			expectedPayload:        []byte("some testing text"),
			expectedSetupEncError:  false,
			expectedSetupDecrError: false,
			expectedEncError:       false,
			expectedDecrError:      false,
		},
		{
			name:                   "large",
			privateKey:             priv,
			publicKey:              pub,
			payload:                []byte(strings.Repeat("t", 1000)),
			expectedPayload:        []byte(strings.Repeat("t", 1000)),
			expectedSetupEncError:  false,
			expectedSetupDecrError: false,
			expectedEncError:       false,
			expectedDecrError:      false,
		},
		{
			name:                   "nil payload",
			privateKey:             priv,
			publicKey:              pub,
			payload:                nil,
			expectedPayload:        nil,
			expectedSetupEncError:  false,
			expectedSetupDecrError: false,
			expectedEncError:       false,
			expectedDecrError:      false,
		},
		{
			name:                   "nil public",
			privateKey:             priv,
			publicKey:              nil,
			payload:                []byte("some testing text"),
			expectedPayload:        []byte("some testing text"),
			expectedSetupEncError:  true,
			expectedSetupDecrError: false,
			expectedEncError:       false,
			expectedDecrError:      false,
		},
		{
			name:                   "nil private",
			privateKey:             nil,
			publicKey:              pub,
			payload:                []byte("some testing text"),
			expectedPayload:        []byte("some testing text"),
			expectedSetupEncError:  false,
			expectedSetupDecrError: true,
			expectedEncError:       false,
			expectedDecrError:      false,
		},
		{
			name:                   "empty payload",
			privateKey:             priv,
			publicKey:              pub,
			payload:                []byte{},
			expectedPayload:        []byte{},
			expectedSetupEncError:  false,
			expectedSetupDecrError: false,
			expectedEncError:       false,
			expectedDecrError:      false,
		},
		{
			name:                   "corrupted ciphertext",
			privateKey:             priv,
			publicKey:              pub,
			payload:                []byte("some testing text"),
			expectedPayload:        nil,
			expectedSetupEncError:  false,
			expectedSetupDecrError: false,
			expectedEncError:       false,
			expectedDecrError:      true, // Corrupt data will cause decryption failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure global func var is reset before test
			EncryptInnerPayload = nil
			DecryptInnerPayload = nil

			err := SetupDecryptInnerPayload(tt.privateKey)
			if err != nil && !tt.expectedSetupDecrError {
				t.Fatalf("expected no decrypt setup error, but got '%v'", err)
			}
			if err == nil && tt.expectedSetupDecrError {
				t.Fatalf("expected decrypt setup error, but got nil")
			}
			err = SetupEncryptInnerPayload(tt.publicKey)
			if err != nil && !tt.expectedSetupEncError {
				t.Fatalf("expected no encrypt setup error, but got '%v'", err)
			}
			if err == nil && tt.expectedSetupEncError {
				t.Fatalf("expected encrypt setup error, but got nil")
			}

			// Expected setup errors, continue to next test
			if tt.expectedSetupDecrError || tt.expectedSetupEncError {
				return
			}

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
