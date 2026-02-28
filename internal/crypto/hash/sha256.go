package hash

import (
	"crypto/sha256"
)

// Creates SHA256 hash of supplied data.
func SHA256(input []byte) (hash [32]byte) {
	hash = sha256.Sum256(input)
	return
}
