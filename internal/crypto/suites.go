package crypto

import (
	"golang.org/x/crypto/chacha20poly1305"
)

type SuiteInfo struct {
	Name           string
	KeySize        int
	NonceSize      int
	CipherOverhead int
}

// Byte length for ID in blobs
const SuiteIDLen int = 1

// Fixed array of 256 slots, indexed by suite ID
var cryptoSuites [256]*SuiteInfo

func init() {
	cryptoSuites[0] = &SuiteInfo{
		Name:           "testing",
		KeySize:        0,
		NonceSize:      0,
		CipherOverhead: 0,
	}
	cryptoSuites[1] = &SuiteInfo{
		Name:           "x25519-hkdf-chacha20poly1305",
		KeySize:        chacha20poly1305.KeySize,
		NonceSize:      chacha20poly1305.NonceSize,
		CipherOverhead: chacha20poly1305.Overhead,
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
