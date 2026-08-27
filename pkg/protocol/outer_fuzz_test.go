package protocol

import (
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/pkg/crypto/registry"
	"strings"
	"testing"
)

func FuzzDeconstructOuterPayload(f *testing.F) {
	seedSuiteInfo, isValidSuite := registry.GetSuiteInfo(1)
	if !isValidSuite {
		f.Fatalf("suite ID 1 not registered")
	}

	_, eachPublicKey, newKeyErr := seedSuiteInfo.NewKey()
	if newKeyErr != nil {
		f.Fatalf("failed to generate seed keys: %v", newKeyErr)
	}

	err := wrappers.SetupEncryptInnerPayload(eachPublicKey)
	if err != nil {
		f.Fatalf("failed to setup seed encryption: %v", err)
	}

	seedPayload := []byte("fuzz-seed-payload-" + strings.Repeat("a", minInnerPayloadLen))
	validOuterPayload, constructErr := ConstructOuterPayload(seedPayload, 1)
	if constructErr != nil {
		f.Fatalf("failed to construct valid seed payload: %v", constructErr)
	}

	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x01})
	f.Add([]byte{0xFF})
	f.Add(validOuterPayload)

	minHeaderLength := registry.SuiteIDLen + seedSuiteInfo.KeySize + seedSuiteInfo.NonceSize
	minHeaderBlob := make([]byte, minHeaderLength)
	for eachIndex := range minHeaderBlob {
		minHeaderBlob[eachIndex] = 0x01
	}
	f.Add(minHeaderBlob)

	truncatedBlob := make([]byte, minHeaderLength-1)
	for eachIndex := range len(truncatedBlob) {
		truncatedBlob[eachIndex] = 0x01
	}
	f.Add(truncatedBlob)

	longZeros := make([]byte, 1024)
	f.Add(longZeros)

	overSizeBlob := make([]byte, 65536)
	f.Add(overSizeBlob)

	eachCorpusBytes := append([]byte{0x00}, validOuterPayload[1:]...)
	f.Add(eachCorpusBytes)

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzSuiteInfo, fuzzValidSuite := registry.GetSuiteInfo(1)
		if !fuzzValidSuite {
			return
		}

		fuzzPrivateKey, _, fuzzNewKeyErr := fuzzSuiteInfo.NewKey()
		if fuzzNewKeyErr != nil {
			return
		}

		fuzzDecryptErr := wrappers.SetupDecryptInnerPayload(fuzzPrivateKey)
		if fuzzDecryptErr != nil {
			return
		}

		_, _ = DeconstructOuterPayload(data)
	})
}
