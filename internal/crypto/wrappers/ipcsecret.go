package wrappers

import (
	"fmt"
	"sdsyslog/internal/crypto/hkdf"
	"sdsyslog/pkg/fipr"
)

// Retrieves shared secret for whole program use (and inter-process communication).
// Secret is derived using known private key through a key derivation function at program startup.
var GetSharedSecret func() (programSecret []byte) = getSharedSecretSafeFail

// Ensure uninitialized function produces panic when called
func getSharedSecretSafeFail() (programSecret []byte) {
	panic("function GetSharedSecret was not initialized")
}

// Sets up shared secret getter function injecting the derived secret from the private key to the local scope.
// Does not set the function if key is empty. Will throw error if function is not initialized and no key is provided.
func SetupGetSharedSecret(serverPriv []byte) (err error) {
	if len(serverPriv) <= 0 {
		if GetSharedSecret == nil {
			err = fmt.Errorf("provided no private key and shared secret function is not already initialized")
		}
		return
	}

	// Derivation will zero slice, so make a copy
	privCopy := make([]byte, len(serverPriv))
	copy(privCopy, serverPriv)

	newSecret, err := hkdf.DeriveKey(privCopy, []byte("fipr-salt"), "fipr-secret", fipr.HMACSize)
	if err != nil {
		err = fmt.Errorf("failed to derive a secret from provided private key: %w", err)
		return
	}

	GetSharedSecret = func() (secret []byte) {
		secret = make([]byte, len(newSecret))
		copy(secret, newSecret)
		return
	}
	return
}
