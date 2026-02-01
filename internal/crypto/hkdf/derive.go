package hkdf

import (
	"crypto/sha512"
	"fmt"
	"sdsyslog/internal/crypto"

	"golang.org/x/crypto/hkdf"
)

// Derives a more secure key from a high entropy shared secret and salt
// Adds namespacing to the derived key
// Returned key length is of the keySize input
// Secret and salt is zeroed after key creation
func DeriveKey(secret, salt []byte, namespace string, keySize int) (secureKey []byte, err error) {
	info := []byte(namespace)
	deriver := hkdf.New(sha512.New, secret, salt, info)

	secureKey = make([]byte, keySize)

	_, err = deriver.Read(secureKey)
	crypto.Memzero(salt)   // Kill salt memory
	crypto.Memzero(secret) // Kill secret memory
	if err != nil {
		err = fmt.Errorf("failed to populate key with secure bytes: %w", err)
		return
	}
	return
}
