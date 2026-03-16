package registry

import (
	"bytes"
	"testing"
)

func TestSignatureSuites(t *testing.T) {
	tests := []struct {
		id                  uint8
		name                string
		expectValid         bool
		expectKeyCreateFail bool
		expectVerifyFail    bool
	}{
		{
			id:                  0,
			name:                "NoSignature",
			expectValid:         true,
			expectKeyCreateFail: true,
		},
		{
			id:          1,
			name:        "ed25519",
			expectValid: true,
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
			info, valid := GetSignatureInfo(tt.id)

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
			sig, err := info.Sign(priv, msg)
			if err != nil {
				t.Fatalf("Sign failed: %v", err)
			}

			// Signature length bounds
			if len(sig) < info.MinSignatureLength || len(sig) > info.MaxSignatureLength {
				t.Fatalf("signature length out of bounds: %d (min=%d max=%d)",
					len(sig), info.MinSignatureLength, info.MaxSignatureLength)
			}

			// Verification success
			if !info.Verify(pub, msg, sig) {
				t.Fatalf("valid signature failed verification")
			}

			if tt.expectVerifyFail {
				return
			}

			// Verification failure (wrong message)
			if info.Verify(pub, wrongMsg, sig) {
				t.Fatalf("verification should fail for wrong message")
			}

			// Verification failure (tampered signature)
			if len(sig) > 0 {
				badSig := bytes.Clone(sig)
				badSig[0] ^= 0xFF

				if info.Verify(pub, msg, badSig) {
					t.Fatalf("verification should fail for corrupted signature")
				}
			}
		})
	}
}
