package registry

import (
	"crypto/ed25519"
	"fmt"
	"sdsyslog/internal/crypto/random"
)

// Fixed array of 256 slots, indexed by suite ID
var signatureSuites [256]*SigInfo

func init() {
	signatureSuites[0] = &SigInfo{
		Name:               "NoSignature",
		MinSignatureLength: 0,
		MaxSignatureLength: 0,
		ValidateKey: func(key []byte) (err error) {
			// If called with a key, produce error to ensure failed signing efforts are not masked
			if len(key) > 0 {
				err = fmt.Errorf("attempt to validate non-empty signing key is invalid for signature suite ID 0")
			}
			return
		},
		Sign:   func([]byte, []byte) ([]byte, error) { return nil, nil },
		Verify: func([]byte, []byte, []byte) bool { return true }, // Intentional verify (pass-through)
		NewKey: func() (priv []byte, pub []byte, err error) {
			err = fmt.Errorf("attempt to create new signing keys is invalid for signature suite ID 0")
			return
		},
	}
	signatureSuites[1] = &SigInfo{
		Name:               "ed25519",
		MinSignatureLength: ed25519.SignatureSize, // Fixed length sig
		MaxSignatureLength: ed25519.SignatureSize, // Fixed length sig
		ValidateKey: func(key []byte) (err error) {
			if len(key) != ed25519.PrivateKeySize {
				err = fmt.Errorf("ed25519 private key must be %d bytes (got %d bytes)", ed25519.PrivateKeySize, len(key))
				return
			}
			return
		},
		Sign: func(priv []byte, msg []byte) (signature []byte, err error) {
			if len(priv) != ed25519.PrivateKeySize {
				err = fmt.Errorf("invalid ed25519 private key size: must be %d (provided key is %d bytes)", ed25519.PrivateKeySize, len(priv))
				return
			}
			signature = ed25519.Sign(ed25519.PrivateKey(priv), msg)
			return
		},
		Verify: func(pub []byte, msg []byte, sig []byte) (valid bool) {
			if len(pub) != ed25519.PublicKeySize ||
				len(sig) != ed25519.SignatureSize {
				return
			}
			valid = ed25519.Verify(ed25519.PublicKey(pub), msg, sig)
			return
		},
		NewKey: func() (privateKey []byte, publicKey []byte, err error) {
			var newSeed []byte
			err = random.PopulateEmptySlice(&newSeed, ed25519.SeedSize)
			if err != nil {
				err = fmt.Errorf("failed to create key seed: %w", err)
			}
			priv := ed25519.NewKeyFromSeed(newSeed)
			pub, ok := priv.Public().(ed25519.PublicKey)
			if !ok {
				err = fmt.Errorf("failed to type assert public key generic to ed25519.PublicKey []byte: value=%+v type=%T", priv.Public(), priv.Public())
				return
			}
			privateKey = priv
			publicKey = pub
			return
		},
	}
}

// Query signature suite (concurrent safe)
func GetSignatureInfo(id uint8) (info SigInfo, valid bool) {
	sig := signatureSuites[id]
	if sig == nil {
		return
	}
	info = *sig
	valid = true
	return
}
