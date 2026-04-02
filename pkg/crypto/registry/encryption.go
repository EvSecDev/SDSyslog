package registry

import (
	"crypto/rand"
	"fmt"
	"sdsyslog/internal/crypto/aead"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/crypto/hkdf"
	"slices"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// Fixed array of 256 slots, indexed by suite ID
var cryptoSuites [256]*SuiteInfo

func init() {
	const (
		testEphPub string = "public"
		testNonce  string = "nonce"
	)

	cryptoSuites[0] = &SuiteInfo{
		Name:           "testing",
		KeySize:        len(testEphPub),
		NonceSize:      len(testNonce),
		CipherOverhead: 0, // No Tag
		ValidateKey: func(key []byte) (err error) {
			if len(key) > 0 {
				err = fmt.Errorf("attempt to validate non-empty encryption key is invalid for crypto suite ID 0")
			}
			return
		},
		NewKey: func() (priv []byte, pub []byte, err error) {
			// Empty keys is valid
			return
		},
		Encrypt: func(publicKey, payload []byte) (ciphertext []byte, ephemeralPub []byte, nonce []byte, err error) {
			// Pass through
			_ = publicKey
			ciphertext = payload
			ephemeralPub = []byte(testEphPub)
			nonce = []byte(testNonce)
			return
		},
		Decrypt: func(privateKey, ciphertext, ephemeralPub, nonce []byte) (payload []byte, err error) {
			// Pass through
			_ = privateKey
			payload = ciphertext
			if !slices.Equal(ephemeralPub, []byte(testEphPub)) {
				err = fmt.Errorf("invalid ephemeral test public %q (must be 'public')", string(ephemeralPub))
				return
			}
			if !slices.Equal(nonce, []byte(testNonce)) {
				err = fmt.Errorf("invalid nonce test %q (must be 'nonce')", string(nonce))
				return
			}
			return
		},
	}
	cryptoSuites[1] = &SuiteInfo{
		Name:           "x25519-hkdf-chacha20poly1305",
		KeySize:        curve25519.ScalarSize, // Outer payload (ephemeral key size)
		NonceSize:      chacha20poly1305.NonceSize,
		CipherOverhead: chacha20poly1305.Overhead,
		ValidateKey: func(key []byte) (err error) {
			if len(key) != curve25519.ScalarSize {
				err = fmt.Errorf("persistent private key must be %d bytes (got %d bytes)", curve25519.ScalarSize, len(key))
				return
			}
			return
		},
		NewKey: func() (privateKey []byte, publicKey []byte, err error) {
			privateKey, publicKey, err = ecdh.CreatePersistentKey()
			return
		},
		DerivePublicKey: func(privateKey []byte) (publicKey []byte, err error) {
			publicKey, err = ecdh.DerivePersistentPublicKey(privateKey)
			return
		},
		Encrypt: func(publicKey, payload []byte) (ciphertext []byte, ephemeralPub []byte, nonce []byte, err error) {
			sharedSecret, ephemeralPub, err := ecdh.CreateSharedSecret(publicKey)
			if err != nil {
				err = fmt.Errorf("failed to create secret: %w", err)
				return
			}

			nonce = make([]byte, chacha20poly1305.NonceSize)
			_, err = rand.Read(nonce)
			if err != nil {
				err = fmt.Errorf("failed to create random nonce: %w", err)
				return
			}

			salt, err := hash.MultipleSlices(ephemeralPub, nonce)
			if err != nil {
				err = fmt.Errorf("failed to create salt: %w", err)
				return
			}

			actualKey, err := hkdf.DeriveKey(sharedSecret,
				salt,
				"x25519-hkdf-chacha20poly1305",
				chacha20poly1305.KeySize)
			if err != nil {
				err = fmt.Errorf("failed to derive key: %w", err)
				return
			}

			aad := append([]byte{1}, ephemeralPub...)
			ciphertext, err = aead.Encrypt(payload, actualKey, nonce, aad)
			if err != nil {
				err = fmt.Errorf("failed encryption: %w", err)
				return
			}
			return
		},
		Decrypt: func(privateKey, ciphertext, ephemeralPub, nonce []byte) (payload []byte, err error) {
			sharedSecret, err := ecdh.ReCreateSharedSecret(privateKey, ephemeralPub)
			if err != nil {
				err = fmt.Errorf("failed recreating secret: %w", err)
				return
			}

			salt, err := hash.MultipleSlices(ephemeralPub, nonce)
			if err != nil {
				err = fmt.Errorf("failed creating salt: %w", err)
				return
			}

			actualKey, err := hkdf.DeriveKey(sharedSecret,
				salt,
				"x25519-hkdf-chacha20poly1305",
				chacha20poly1305.KeySize)
			if err != nil {
				err = fmt.Errorf("failed deriving key: %w", err)
				return
			}

			aad := append([]byte{1}, ephemeralPub...)
			payload, err = aead.Decrypt(ciphertext, actualKey, nonce, aad)
			if err != nil {
				return
			}
			return
		},
	}
}

// Query crypto suite (concurrent safe)
func GetSuiteInfo(id uint8) (info SuiteInfo, valid bool) {
	suite := cryptoSuites[id]
	if suite == nil {
		return
	}
	info = *suite
	valid = true
	return
}
