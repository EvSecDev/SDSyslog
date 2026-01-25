package wrappers

import (
	"crypto/rand"
	"fmt"
	"sdsyslog/internal/crypto"
	"sdsyslog/internal/crypto/aead"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/crypto/hkdf"
)

// Wrapper to encrypt an inner payload into cipher text
var EncryptInnerPayload func(payload []byte, suiteID uint8) (ciphertext, ephemeralPub, nonce []byte, err error)

// Wrapper to decrypt an inner payload from cipher text to clear text
var DecryptInnerPayload func(ciphertext, ephemeralPub, nonce []byte, suiteID uint8) (innerPayload []byte, err error)

// Sets up encryption wrapper function injecting the public key to the local scope.
// Does not set the function if key is empty. Will throw error if function is not initialized and no key is provided.
func SetupEncryptInnerPayload(serverPub []byte) (err error) {
	if len(serverPub) <= 0 {
		if EncryptInnerPayload == nil {
			err = fmt.Errorf("provided no public key and encryption function is not already initialized")
		}
		return
	}

	EncryptInnerPayload = func(payload []byte, suiteID uint8) (ciphertext, ephemeralPub, nonce []byte, err error) {
		if len(serverPub) == 0 {
			err = fmt.Errorf("public key empty: attempted call to uninitialized function")
			return
		}

		sharedSecret, ephemeralPub, err := ecdh.CreateSharedSecret(serverPub)
		if err != nil {
			err = fmt.Errorf("failed to create secret: %v", err)
			return
		}

		suite, _ := crypto.GetSuiteInfo(1)
		nonce = make([]byte, suite.NonceSize)
		_, err = rand.Read(nonce)
		if err != nil {
			err = fmt.Errorf("failed to create random nonce: %v", err)
			return
		}

		salt, err := hash.MultipleSlices(ephemeralPub, nonce)
		if err != nil {
			err = fmt.Errorf("failed to create salt: %v", err)
			return
		}

		actualKey, err := hkdf.DeriveKey(sharedSecret, salt, suite.Name, suite.KeySize)
		if err != nil {
			err = fmt.Errorf("failed to derive key: %v", err)
			return
		}

		aad := append([]byte{suiteID}, ephemeralPub...)
		ciphertext, err = aead.Encrypt(payload, actualKey, nonce, aad)
		if err != nil {
			err = fmt.Errorf("failed encryption: %v", err)
			return
		}

		return
	}
	return
}

// Sets up decryption wrapper function injecting the private key to the local scope.
// Does not set the function if key is empty. Will throw error if function is not initialized and no key is provided.
func SetupDecryptInnerPayload(privateKey []byte) (err error) {
	if len(privateKey) <= 0 {
		if DecryptInnerPayload == nil {
			err = fmt.Errorf("provided no public key and encryption function is not already initialized")
		}
		return
	}

	DecryptInnerPayload = func(ciphertext, ephemeralPub, nonce []byte, suiteID uint8) (innerPayload []byte, err error) {
		if len(privateKey) == 0 {
			err = fmt.Errorf("private key empty: attempted call to uninitialized function")
			return
		}

		sharedSecret, err := ecdh.ReCreateSharedSecret(privateKey, ephemeralPub)
		if err != nil {
			err = fmt.Errorf("failed recreating secret: %v", err)
			return
		}

		salt, err := hash.MultipleSlices(ephemeralPub, nonce)
		if err != nil {
			err = fmt.Errorf("failed creating salt: %v", err)
			return
		}

		suite, _ := crypto.GetSuiteInfo(suiteID)
		actualKey, err := hkdf.DeriveKey(sharedSecret, salt, suite.Name, suite.KeySize)
		if err != nil {
			err = fmt.Errorf("failed deriving key: %v", err)
			return
		}

		aad := append([]byte{suiteID}, ephemeralPub...)
		innerPayload, err = aead.Decrypt(ciphertext, actualKey, nonce, aad)
		if err != nil {
			return
		}

		return
	}
	return
}
