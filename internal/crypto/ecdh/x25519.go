package ecdh

import (
	"crypto/rand"
	"fmt"
	"sdsyslog/internal/crypto"

	"golang.org/x/crypto/curve25519"
)

// Fixed key length for x25519
const KeyLen int = 32

// Creates asymmetric key pair using x25519
func CreatePersistentKey() (private, public []byte, err error) {
	// Create a secure random 32-byte private key
	private = make([]byte, KeyLen)
	_, err = rand.Read(private)
	if err != nil {
		err = fmt.Errorf("failed to generate random private key: %w", err)
		return
	}

	public, err = DerivePersistentPublicKey(private)
	if err != nil {
		return
	}

	return
}

// Using existing asymmetric private key (x25519) to derive the associated public key
func DerivePersistentPublicKey(private []byte) (public []byte, err error) {
	public, err = curve25519.X25519(private, curve25519.Basepoint)
	if err != nil {
		err = fmt.Errorf("failed to derive public key: %w", err)
		return
	}
	return
}

// Uses supplied public key to derive a shared secret and ephemeral public key
// Meant for use on sender side
func CreateSharedSecret(publicKey []byte) (sharedSecret, ephemeralPublic []byte, err error) {
	// Ephemeral key pair for this message
	ephemeralPriv := make([]byte, KeyLen)
	_, err = rand.Read(ephemeralPriv)
	if err != nil {
		err = fmt.Errorf("failed to generate ephemeral private key: %w", err)
		return
	}

	// Create the ephemeral public key
	ephemeralPublic, err = curve25519.X25519(ephemeralPriv, curve25519.Basepoint)
	if err != nil {
		err = fmt.Errorf("failed to generate ephemeral public key: %w", err)
		return
	}

	// Create shared secret based on persistent public key
	sharedSecret, err = curve25519.X25519(ephemeralPriv, publicKey)
	crypto.Memzero(ephemeralPriv) // Kill ephemeral private memory
	if err != nil {
		err = fmt.Errorf("failed to compute shared secret: %w", err)
		return
	}

	return
}

// Uses supplied private key and ephemeral public to re-derive shared secret
// Meant for use on receiver side
func ReCreateSharedSecret(privateKey, ephemeralPublic []byte) (sharedSecret []byte, err error) {
	sharedSecret, err = curve25519.X25519(privateKey, ephemeralPublic)
	if err != nil {
		err = fmt.Errorf("failed to recompute shared secret: %w", err)
		return
	}
	return
}
