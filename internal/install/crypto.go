package install

import (
	"encoding/base64"
	"fmt"
	"sdsyslog/internal/crypto/ecdh"
)

func GeneratePrivateKeys() (err error) {
	private, public, err := ecdh.CreatePersistentKey()
	if err != nil {
		return
	}

	fmt.Printf("Private Key: %s\n", base64.StdEncoding.EncodeToString(private))
	fmt.Printf("Public Key: %s\n", base64.StdEncoding.EncodeToString(public))
	return
}
