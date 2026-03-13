package install

import (
	"encoding/base64"
	"fmt"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/pkg/crypto/registry"
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

func GenerateSigningKeys(suiteID uint8) (err error) {
	info, validID := registry.GetSignatureInfo(suiteID)
	if !validID {
		err = fmt.Errorf("invalid suite ID: %d", suiteID)
		return
	}
	priv, pub, err := info.NewKey()
	if err != nil {
		err = fmt.Errorf("failed to generate new key pair: %w", err)
		return
	}
	fmt.Printf("Private Key: %s\n", base64.StdEncoding.EncodeToString(priv))
	fmt.Printf("Public Key: %s\n", base64.StdEncoding.EncodeToString(pub))
	return
}
