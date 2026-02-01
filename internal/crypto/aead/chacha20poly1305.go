package aead

import (
	"fmt"
	"sdsyslog/internal/crypto"
	"sdsyslog/internal/crypto/random"

	"golang.org/x/crypto/chacha20poly1305"
)

// Encrypts provided plain text using provided parameters using chacha20poly1305 AEAD cipher
// Zeroes key memory after encryption
func Encrypt(plaintext, key, nonce, additional []byte) (ciphertext []byte, err error) {
	err = random.PopulateEmptySlice(&nonce, chacha20poly1305.NonceSize)
	if err != nil {
		err = fmt.Errorf("encountered error fixing insecure provided nonce: %w", err)
		return
	}

	// Create Cipher from key
	aead, err := chacha20poly1305.New(key)
	crypto.Memzero(key) // Kill keys' memory
	if err != nil {
		err = fmt.Errorf("failed creation of AEAD: %w", err)
		return
	}

	// Encrypt message
	ciphertext = aead.Seal(nil, nonce, plaintext, additional)
	// Do not zero nonce memory - must be included post-encryption
	return
}

// Decrypts provided cipher text using provided parameters using chacha20poly1305 AEAD cipher
// Zeroes both key and nonce after decryption
func Decrypt(ciphertext, key, nonce, additional []byte) (plaintext []byte, err error) {
	// Create Cipher from key
	aead, err := chacha20poly1305.New(key)
	crypto.Memzero(key) // Kill keys' memory
	if err != nil {
		err = fmt.Errorf("failed creation of AEAD: %w", err)
		return
	}

	// Decrypt message
	plaintext, err = aead.Open(nil, nonce, ciphertext, additional)
	crypto.Memzero(nonce) // Kill nonces' memory
	if err != nil {
		err = fmt.Errorf("failed decryption of cipher text: %w", err)
		return
	}

	return
}
