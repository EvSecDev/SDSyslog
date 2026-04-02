// Package to act as an entry point for generic cryptographic operations (wraps crypto/sig registry operations)
package wrappers

import (
	"fmt"
	"sdsyslog/internal/crypto"
	"sdsyslog/pkg/crypto/registry"
)

// Wrapper to encrypt an inner payload into cipher text
var EncryptInnerPayload func(payload []byte, suiteID uint8) (ciphertext, ephemeralPub, nonce []byte, err error) = encryptInnerSafeFail

// Ensure uninitialized function produces an error when called
func encryptInnerSafeFail(payload []byte, suiteID uint8) (ciphertext, ephemeralPub, nonce []byte, err error) {
	_ = payload
	_ = suiteID
	err = fmt.Errorf("function EncryptInnerPayload was not initialized")
	return
}

// Wrapper to decrypt an inner payload from cipher text to clear text
var DecryptInnerPayload func(ciphertext, ephemeralPub, nonce []byte, suiteID uint8) (innerPayload []byte, err error) = decryptInnerSafeFail

// Ensure uninitialized function produces an error when called
func decryptInnerSafeFail(ciphertext, ephemeralPub, nonce []byte, suiteID uint8) (innerPayload []byte, err error) {
	_ = ciphertext
	_ = ephemeralPub
	_ = nonce
	_ = suiteID
	err = fmt.Errorf("function DecryptInnerPayload was not initialized")
	return
}

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

		if crypto.IsZero(serverPub) {
			err = fmt.Errorf("public key empty: all bytes are zero")
			return
		}

		suite, valid := registry.GetSuiteInfo(suiteID)
		if !valid {
			err = fmt.Errorf("invalid crypto suite ID %d", suiteID)
			return
		}

		ciphertext, ephemeralPub, nonce, err = suite.Encrypt(serverPub, payload)
		return
	}
	return
}

// Sets up decryption wrapper function injecting the private key to the local scope.
// Does not set the function if key is empty. Will throw error if function is not initialized and no key is provided.
func SetupDecryptInnerPayload(privateKey []byte) (err error) {
	if len(privateKey) <= 0 {
		if DecryptInnerPayload == nil {
			err = fmt.Errorf("provided no private key and encryption function is not already initialized")
		}
		return
	}

	DecryptInnerPayload = func(ciphertext, ephemeralPub, nonce []byte, suiteID uint8) (innerPayload []byte, err error) {
		if len(privateKey) == 0 {
			err = fmt.Errorf("private key empty: attempted call to uninitialized function")
			return
		}

		if crypto.IsZero(privateKey) {
			err = fmt.Errorf("private key empty: all bytes are zero")
			return
		}

		suite, valid := registry.GetSuiteInfo(suiteID)
		if !valid {
			err = fmt.Errorf("invalid crypto suite ID %d", suiteID)
			return
		}

		innerPayload, err = suite.Decrypt(privateKey, ciphertext, ephemeralPub, nonce)
		return
	}
	return
}
