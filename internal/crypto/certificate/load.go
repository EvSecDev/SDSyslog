// Central package for certificate related functions
package certificate

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// Parses x509 PEM certificate file for a public key (only supports ed25519 and ecdsa)
func LoadPublicKeyFile(path string) (publicKey []byte, algoType x509.PublicKeyAlgorithm, err error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		err = fmt.Errorf("failed to decode PEM")
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return
	}
	publicKey, algoType, err = parseCertificate(cert)
	return
}

// Parses x509 for individual public key and algorithm
func parseCertificate(cert *x509.Certificate) (publicKey []byte, algoType x509.PublicKeyAlgorithm, err error) {
	algoType = cert.PublicKeyAlgorithm

	switch pub := cert.PublicKey.(type) {
	case ed25519.PublicKey:
		var ok bool
		publicKey, ok = cert.PublicKey.(ed25519.PublicKey)
		if !ok {
			err = fmt.Errorf("certificate public key is not ed25519 public key: value=%+v type=%T", cert.PublicKey, cert.PublicKey)
			return
		}
	case *ecdsa.PublicKey:
		pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			err = fmt.Errorf("public key is not ECDSA: value=%+v type=%T", cert.PublicKey, cert.PublicKey)
			return
		}
		publicKey, err = x509.MarshalPKIXPublicKey(pubKey)
		if err != nil {
			return
		}
	default:
		err = fmt.Errorf("unsupported public key type: %T", pub)
	}
	return
}
