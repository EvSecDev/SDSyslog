package wrappers

import (
	"fmt"
	"sdsyslog/internal/crypto"
	"sdsyslog/pkg/crypto/registry"
	"sync/atomic"
)

// Wrapper to sign a byte slice per the signature ID algorithm.
// Will return empty signature slice when sig is 0 (Safe to call when no signature is desired).
var CreateSignature func(data []byte, sigID uint8) (signature []byte, err error) = createSignatureSafeFail

// Ensure uninitialized function produces an error when called
func createSignatureSafeFail(data []byte, sigID uint8) (signature []byte, err error) {
	_ = data
	_ = sigID
	err = fmt.Errorf("function CreateSignature was not initialized")
	return
}

// Wrapper to verify a signature of provided data per signature ID algorithm.
// Will return invalid when signature ID is 0 (no signature)
var VerifySignature func(publicKey, data, signature []byte, sigID uint8) (valid bool, err error) = verifySignatureSafeFail

// Ensure uninitialized function produces an error when called
func verifySignatureSafeFail(publicKey, data, signature []byte, sigID uint8) (valid bool, err error) {
	_ = publicKey
	_ = signature
	_ = data
	_ = sigID
	err = fmt.Errorf("function VerifySignature was not initialized")
	return
}

// Sets up signature creation wrapper function injecting the private key to the local scope.
// Does not set the function if key is empty. Will throw error if function is not initialized and no key is provided.
func SetupCreateSignature(privateKey []byte) (err error) {
	if len(privateKey) <= 0 {
		if CreateSignature == nil {
			err = fmt.Errorf("provided no private key and create signature function is not already initialized")
		}
		return
	}

	CreateSignature = func(data []byte, sigID uint8) (signature []byte, err error) {
		if crypto.IsZero(privateKey) {
			err = fmt.Errorf("private key empty: all bytes are zero")
			return
		}

		sigSuite, valid := registry.GetSignatureInfo(sigID)
		if !valid {
			err = fmt.Errorf("unknown signature ID: %d", sigID)
			return
		}

		if sigID == 0 {
			// Signature is purposely empty slice when using 0 id
			return
		} else {
			if len(privateKey) == 0 {
				err = fmt.Errorf("private key empty: attempted call to uninitialized function")
				return
			}
		}

		signature, err = sigSuite.Sign(privateKey, data)
		if err != nil {
			return
		}

		return
	}
	return
}

var pinnedSenderKeys atomic.Value // stores map[string][]byte (map[hostname]publicKey)

// Set new pinned sender keys map (concurrent safe)
func NewPinnedSenders(keys map[string][]byte) {
	// Make a copy so we own it
	pinnedKeys := make(map[string][]byte, len(keys))
	for hostname, key := range keys {
		newKey := make([]byte, len(key))
		copy(newKey, key)

		pinnedKeys[hostname] = newKey
	}
	pinnedSenderKeys.Store(pinnedKeys)
}

// Retrieves key for hostname if present.
// Present is false when hostname is not known in pinned keys map or pinned map does not exist/empty.
func LookupPinnedSender(hostname string) (key []byte, present bool) {
	keyMap := pinnedSenderKeys.Load()
	if keyMap == nil {
		return
	}
	pinnedKeys, ok := keyMap.(map[string][]byte)
	if !ok {
		present = false
		return
	}
	if len(pinnedKeys) == 0 {
		return
	}
	key, ok = pinnedKeys[hostname]
	if !ok {
		present = false
		return
	} else {
		present = true
	}
	return
}

// Sets up signature verification wrapper function injecting the public key map to the local scope.
// Does not set the function if key is empty. Will throw error if function is not initialized and no key is provided.
func SetupVerifySignature(keys map[string][]byte) (err error) {
	NewPinnedSenders(keys)
	VerifySignature = func(publicKey, data, signature []byte, sigID uint8) (valid bool, err error) {
		if sigID == 0 {
			valid = false
			err = fmt.Errorf("verification cannot occur with signature ID 0")
			return
		}

		if crypto.IsZero(publicKey) {
			err = fmt.Errorf("public key empty: all bytes are zero")
			return
		}

		sigSuite, valid := registry.GetSignatureInfo(sigID)
		if !valid {
			err = fmt.Errorf("unknown signature ID: %d", sigID)
			return
		}

		valid = sigSuite.Verify(publicKey, data, signature)
		return
	}
	return
}
