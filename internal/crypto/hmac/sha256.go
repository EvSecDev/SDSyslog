package hmac

import (
	"crypto/hmac"
	"crypto/sha256"
)

func ComputeSHA256(key []byte, desiredSize int, data []byte) (mac []byte) {
	hash := hmac.New(sha256.New, key)

	hash.Write(data)
	full := hash.Sum(nil)

	mac = full[:desiredSize]
	return
}

func VerifySHA256(key []byte, requiredSize int, data, mac []byte) (valid bool) {
	hash := hmac.New(sha256.New, key)

	hash.Write(data)
	full := hash.Sum(nil)

	valid = hmac.Equal(full[:requiredSize], mac)
	return
}
