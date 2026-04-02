package registry

import (
	"slices"
	"testing"
)

func TestCryptoSuites(t *testing.T) {
	tests := []struct {
		id                  uint8
		name                string
		expectValid         bool
		expectKeyCreateFail bool
		expectDecryptFail   bool
		runTamperTest       bool
	}{
		{
			id:          0,
			name:        "testing",
			expectValid: true,
		},
		{
			id:            1,
			name:          "x25519-hkdf-chacha20poly1305",
			expectValid:   true,
			runTamperTest: true,
		},
		{
			id:          200, // unregistered suite
			name:        "",
			expectValid: false,
		},
	}

	msg := []byte("test message")
	wrongMsg := []byte("wrong message")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, valid := GetSuiteInfo(tt.id)

			if valid != tt.expectValid {
				t.Fatalf("expected valid=%v got %v", tt.expectValid, valid)
			}

			if !valid {
				return
			}

			if info.Name != tt.name {
				t.Fatalf("expected suite name %q got %q", tt.name, info.Name)
			}

			// Key generation
			priv, pub, err := info.NewKey()
			if err != nil {
				if tt.expectKeyCreateFail {
					return
				}
				t.Fatalf("NewKey failed: %v", err)
			}

			// Key validation
			if err := info.ValidateKey(priv); err != nil {
				t.Fatalf("ValidateKey failed: %v", err)
			}

			// Signing
			ciphertext, ephemeralPub, nonce, err := info.Encrypt(pub, msg)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			// Decryption success
			decrMsg, err := info.Decrypt(priv, ciphertext, ephemeralPub, nonce)
			if err != nil && !tt.expectDecryptFail {
				t.Fatalf("valid encryption failed decryption")
			} else if err != nil && tt.expectDecryptFail {
				return
			} else if err == nil && tt.expectDecryptFail {
				t.Fatalf("expected decryption failure, but decrypt did not fail")
			}

			if !slices.Equal(decrMsg, msg) {
				t.Fatalf("expected decrypted message %q to match test message %q, but they differ", string(decrMsg), string(msg))
			}

			if !tt.runTamperTest {
				return
			}

			// Decryption failure (wrong message)
			ciphertext, _, _, err = info.Encrypt(pub, wrongMsg)
			if err != nil {
				t.Fatalf("Encryption for wrong msg test failed: %v", err)
			}

			_, err = info.Decrypt(priv, ciphertext, ephemeralPub, nonce)
			if err == nil {
				t.Fatalf("decryption should fail for wrong message")
			}
		})
	}
}
