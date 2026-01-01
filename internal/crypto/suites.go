package crypto

import (
	"sync"

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

var cryptoSuiteMu sync.Mutex
var cryptoSuiteMap = map[uint8]SuiteInfo{
	0: {
		Name:           "testing",
		KeySize:        0,
		NonceSize:      0,
		CipherOverhead: 0,
	},
	1: {
		Name:           "x25519-hkdf-chacha20poly1305",
		KeySize:        chacha20poly1305.KeySize,
		NonceSize:      chacha20poly1305.NonceSize,
		CipherOverhead: chacha20poly1305.Overhead,
	},
}

// Query crypto suite (concurrent safe)
func GetSuiteInfo(id uint8) (info SuiteInfo, validID bool) {
	cryptoSuiteMu.Lock()
	defer cryptoSuiteMu.Unlock()
	info, validID = cryptoSuiteMap[id]
	return
}
