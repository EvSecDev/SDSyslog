package wrappers

import (
	"bytes"
	"sdsyslog/internal/tests/utils"
	"sdsyslog/pkg/crypto/registry"
	"strings"
	"testing"
)

func TestEncryptionWrapper(t *testing.T) {
	// Force generation of actual keys from registry
	priv := []byte("placeholder")
	pub := []byte("placeholder")

	tests := []struct {
		name                   string
		suiteID                uint8
		privateKey             []byte
		publicKey              []byte
		payload                []byte
		expectedPayload        []byte
		expectedSetupEncError  string
		expectedSetupDecrError string
		expectedEncError       string
		expectedDecrError      string
	}{
		{
			name:            "regular",
			suiteID:         1,
			privateKey:      priv,
			publicKey:       pub,
			payload:         []byte("some testing text"),
			expectedPayload: []byte("some testing text"),
		},
		{
			name:            "large",
			suiteID:         1,
			privateKey:      priv,
			publicKey:       pub,
			payload:         []byte(strings.Repeat("t", 1000)),
			expectedPayload: []byte(strings.Repeat("t", 1000)),
		},
		{
			name:            "nil payload",
			suiteID:         1,
			privateKey:      priv,
			publicKey:       pub,
			payload:         nil,
			expectedPayload: nil,
		},
		{
			name:                  "nil public",
			suiteID:               1,
			privateKey:            priv,
			publicKey:             nil,
			payload:               []byte("some testing text"),
			expectedPayload:       []byte("some testing text"),
			expectedSetupEncError: "provided no public key and encryption function is not already initialized",
		},
		{
			name:                   "nil private",
			suiteID:                1,
			privateKey:             nil,
			publicKey:              pub,
			payload:                []byte("some testing text"),
			expectedPayload:        []byte("some testing text"),
			expectedSetupDecrError: "provided no private key and encryption function is not already initialized",
		},
		{
			name:            "empty payload",
			suiteID:         1,
			privateKey:      priv,
			publicKey:       pub,
			payload:         []byte{},
			expectedPayload: []byte{},
		},
		{
			name:              "corrupted ciphertext",
			suiteID:           1,
			privateKey:        priv,
			publicKey:         pub,
			payload:           []byte("some testing text"),
			expectedPayload:   nil,
			expectedDecrError: "failed decryption of cipher text", // Corrupt data will cause decryption failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure global func var is reset before test
			EncryptInnerPayload = nil
			DecryptInnerPayload = nil

			info, validID := registry.GetSuiteInfo(tt.suiteID)
			if !validID {
				t.Fatalf("invalid suite ID %d", tt.suiteID)
			}
			privKey, pubKey, err := info.NewKey()
			if err != nil {
				t.Fatalf("failed to generate keys: %v", err)
			}

			if !bytes.Equal(tt.privateKey, []byte("placeholder")) {
				privKey = tt.privateKey
			}
			if !bytes.Equal(tt.publicKey, []byte("placeholder")) {
				pubKey = tt.publicKey
			}

			err = SetupDecryptInnerPayload(privKey)
			matches, err := utils.MatchErrorString(err, tt.expectedSetupDecrError)
			if err != nil {
				t.Fatalf("setupDecrypt: %v", err)
			} else if matches {
				return
			}

			err = SetupEncryptInnerPayload(pubKey)
			matches, err = utils.MatchErrorString(err, tt.expectedSetupEncError)
			if err != nil {
				t.Fatalf("setupEncrypt: %v", err)
			} else if matches {
				return
			}

			ciphertext, ephemeralPub, nonce, err := EncryptInnerPayload(tt.payload, tt.suiteID)
			matches, err = utils.MatchErrorString(err, tt.expectedEncError)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			} else if matches {
				return
			}

			// Simulate corruption or error before decryption
			if tt.name == "corrupted ciphertext" {
				ciphertext[0] ^= 0x01 // change a byte
			}

			outPayload, err := DecryptInnerPayload(ciphertext, ephemeralPub, nonce, tt.suiteID)
			matches, err = utils.MatchErrorString(err, tt.expectedDecrError)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			} else if matches {
				return
			}

			if !bytes.Equal(outPayload, tt.expectedPayload) {
				t.Errorf("decrypted output payload does not match expected payload:\n  Expected Payload: %v\n  Actual Payload: %v\n", tt.expectedPayload, outPayload)
			}
		})
	}
}
