package protocol

import (
	"bytes"
	"fmt"
	"sdsyslog/internal/crypto"
	"sdsyslog/internal/crypto/wrappers"
)

// Validate and create transport payload given header fields
func ConstructOuterPayload(innerPayload []byte, suiteID uint8) (outerPayload []byte, err error) {
	// Reject empty payload
	if len(innerPayload) < 1 {
		err = fmt.Errorf("payload cannot be empty")
		return
	}

	// Validate lengths based on requested suiteID
	suite, validID := crypto.GetSuiteInfo(suiteID)
	if !validID {
		err = fmt.Errorf("unknown suite ID %d", suiteID)
		return
	}

	// Encrypt inner payload
	ciphertext, ephemeralPub, nonce, err := wrappers.EncryptInnerPayload(innerPayload, suiteID)
	if err != nil {
		err = fmt.Errorf("failed encryption: %w", err)
		return
	}

	// Validate ephemeral against chosen suite
	if len(ephemeralPub) != suite.KeySize {
		err = fmt.Errorf("invalid key length: suite ID %d requires length %d, but received key length %d", suiteID, suite.KeySize, len(ephemeralPub))
		return
	}

	if len(nonce) != suite.NonceSize {
		err = fmt.Errorf("invalid nonce length: suite ID %d requires length %d, but received nonce length %d", suiteID, suite.NonceSize, len(nonce))
		return
	}

	// Allocate blob length of inputs
	totalLength := crypto.SuiteIDLen + len(ephemeralPub) + len(nonce) + len(ciphertext)
	outerPayload = make([]byte, 0, totalLength)

	// Using buffer to build blob
	var buffer bytes.Buffer

	// Write fields in order
	writeErrMsg := "failed to write field %s to blob buffer"
	err = buffer.WriteByte(suiteID)
	if err != nil {
		err = fmt.Errorf(writeErrMsg, "suiteID")
		return
	}
	_, err = buffer.Write(ephemeralPub)
	if err != nil {
		err = fmt.Errorf(writeErrMsg, "ephemeralPub")
		return
	}
	_, err = buffer.Write(nonce)
	if err != nil {
		err = fmt.Errorf(writeErrMsg, "nonce")
		return
	}
	_, err = buffer.Write(ciphertext)
	if err != nil {
		err = fmt.Errorf(writeErrMsg, "payload")
		return
	}

	// Convert buffer to byte slice
	outerPayload = buffer.Bytes()

	// Kill memory for crypto-related bytes
	crypto.Memzero(ephemeralPub)
	crypto.Memzero(nonce)

	return
}

// Deconstructs and validates transport payload by extracting fields according to embedded suite ID and predefined lengths
func DeconstructOuterPayload(blob []byte) (innerPayload []byte, err error) {
	// Initial check to ensure there's at least one byte
	if len(blob) < 1 {
		err = fmt.Errorf("blob is empty")
		return
	}

	currentIndex := 0 // Running index to manage extraction

	// Immediate extract first byte to check for further parameters
	suiteID := uint8(blob[currentIndex])
	currentIndex += crypto.SuiteIDLen

	// Validate ID is known
	suite, validSuite := crypto.GetSuiteInfo(suiteID)
	if !validSuite {
		err = fmt.Errorf("invalid suite ID (%d) in blob", suiteID)
		return
	}

	// Calculate entire header length to further validate blob
	minLength := crypto.SuiteIDLen + suite.KeySize + suite.NonceSize

	// Reject packets below header length
	if len(blob) < minLength {
		err = fmt.Errorf("suite id %d: blob size is too small: expected minimum size %d but got %d", suiteID, minLength, len(blob))
		return
	}

	// Extract the rest of the fields
	pubKey := blob[currentIndex : currentIndex+suite.KeySize]
	currentIndex += suite.KeySize

	nonce := blob[currentIndex : currentIndex+suite.NonceSize]
	currentIndex += suite.NonceSize

	ciphertext := blob[currentIndex:]

	// Reject invalid minimum length inner payloads
	if len(ciphertext) < minInnerPayloadLen+suite.CipherOverhead {
		err = fmt.Errorf("suite id %d: blob payload is invalid minimum length", suiteID)
		return
	}

	// Decrypt the payload
	innerPayload, err = wrappers.DecryptInnerPayload(ciphertext, pubKey, nonce, suiteID)
	if err != nil {
		return
	}

	return
}
