package wrappers

import (
	"crypto/ed25519"
	"sdsyslog/internal/crypto/random"
	"strings"
	"testing"
)

func TestSignatureWrapper(t *testing.T) {
	var seed []byte
	err := random.PopulateEmptySlice(&seed, ed25519.SeedSize)
	if err != nil {
		t.Fatalf("unexpected error creating random seed: %v", err)
	}
	signingPrivKey := ed25519.NewKeyFromSeed(seed)
	signingPubKey := signingPrivKey.Public().(ed25519.PublicKey)

	suiteID := uint8(1)

	tests := []struct {
		name                     string
		privateKey               []byte
		publicKey                []byte
		payload                  []byte
		expectedVerifySuccess    bool
		expectedSetupCreateError bool
		expectedSetupVerifyError bool
		expectedCreateError      bool
		expectedVerifyError      bool
	}{
		{
			name:                  "regular",
			privateKey:            signingPrivKey,
			publicKey:             signingPubKey,
			payload:               []byte("some testing text"),
			expectedVerifySuccess: true,
		},
		{
			name:                  "large",
			privateKey:            signingPrivKey,
			publicKey:             signingPubKey,
			payload:               []byte(strings.Repeat("t", 1000)),
			expectedVerifySuccess: true,
		},
		{
			name:                  "nil payload",
			privateKey:            signingPrivKey,
			publicKey:             signingPubKey,
			payload:               nil,
			expectedVerifySuccess: true,
		},
		{
			name:                  "empty payload",
			privateKey:            signingPrivKey,
			publicKey:             signingPubKey,
			payload:               []byte{},
			expectedVerifySuccess: true,
		},
		{
			name:                "nil public",
			privateKey:          signingPrivKey,
			publicKey:           nil,
			payload:             []byte("some testing text"),
			expectedVerifyError: true,
		},
		{
			name:                     "nil private",
			privateKey:               nil,
			publicKey:                signingPubKey,
			payload:                  []byte("some testing text"),
			expectedSetupCreateError: true,
		},
		{
			name:                  "corrupted data",
			privateKey:            signingPrivKey,
			publicKey:             signingPubKey,
			payload:               []byte("some testing text"),
			expectedVerifySuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure global func var is reset before test
			CreateSignature = nil
			VerifySignature = nil

			err := SetupCreateSignature(tt.privateKey)
			if err != nil && !tt.expectedSetupCreateError {
				t.Fatalf("expected no create signature setup error, but got '%v'", err)
			}
			if err == nil && tt.expectedSetupCreateError {
				t.Fatalf("expected decrypt setup error, but got nil")
			}
			err = SetupVerifySignature(map[string][]byte{
				"test": tt.publicKey,
			})
			if err != nil && !tt.expectedSetupVerifyError {
				t.Fatalf("expected no encrypt setup error, but got '%v'", err)
			}
			if err == nil && tt.expectedSetupVerifyError {
				t.Fatalf("expected encrypt setup error, but got nil")
			}

			// Expected setup errors, continue to next test
			if tt.expectedSetupVerifyError || tt.expectedSetupCreateError {
				return
			}

			signature, err := CreateSignature(tt.payload, suiteID)
			if err != nil && !tt.expectedCreateError {
				t.Errorf("expected no signing error, but got '%v'", err)
				return
			}
			if err == nil && tt.expectedCreateError {
				t.Errorf("expected signing error, but got no error")
			}
			if tt.expectedCreateError {
				return // signing failures will fail verification
			}

			// Simulate corruption or error before signature verification
			if tt.name == "corrupted data" {
				tt.payload[0] ^= 0x01 // change a byte
			}

			valid, err := VerifySignature(tt.publicKey, tt.payload, signature, suiteID)
			if err != nil && !tt.expectedVerifyError {
				t.Errorf("expected no signature verification error, but got '%v'", err)
				return
			}
			if err == nil && tt.expectedVerifyError {
				t.Errorf("expected signature verification error, but got no error")
			}
			if tt.expectedVerifyError {
				return // signature verification failures will fail content compares
			}

			if valid != tt.expectedVerifySuccess {
				t.Errorf("unexpected signature verification result: expected verify success: %v - got verify success: %v", tt.expectedVerifySuccess, valid)
			}
		})
	}
}
