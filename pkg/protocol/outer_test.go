package protocol

import (
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/wrappers"
	"strings"
	"testing"
)

func TestConstructAndDeconstructouter(t *testing.T) {
	priv, pub, err := ecdh.CreatePersistentKey()
	if err != nil {
		panic(err)
	}

	tests := []struct {
		name        string
		suiteID     uint8
		pubKey      []byte
		payload     []byte
		expectedErr bool
	}{
		{
			name:        "Valid Input",
			suiteID:     1,
			pubKey:      pub,
			payload:     []byte(strings.Repeat("a", minInnerPayloadLen)),
			expectedErr: false,
		},
		{
			name:        "Empty Payload",
			suiteID:     1,
			pubKey:      []byte{0x01, 0x02},
			payload:     nil,
			expectedErr: true,
		},
		{
			name:        "Invalid Key Length",
			suiteID:     1,
			pubKey:      []byte{0x01},
			payload:     []byte("some data"),
			expectedErr: true,
		},
		{
			name:        "Invalid Nonce Length",
			suiteID:     1,
			pubKey:      []byte{0x01, 0x02},
			payload:     []byte("some data"),
			expectedErr: true,
		},
		{
			name:        "Unknown Suite ID",
			suiteID:     9,
			pubKey:      []byte{0x01, 0x02},
			payload:     []byte("valid payload"),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrappers.SetupEncryptInnerPayload(tt.pubKey)
			wrappers.SetupDecryptInnerPayload(priv)

			// Test ConstructOuterPayload
			outerBlob, err := ConstructOuterPayload(tt.payload, tt.suiteID)
			if (err != nil) != tt.expectedErr {
				t.Errorf("ConstructOuterPayload() error = %v, expectedErr %v", err, tt.expectedErr)
			}
			if err == nil && len(outerBlob) == 0 {
				t.Errorf("ConstructOuterPayload() returned an empty blob")
			}

			// Test DeconstructOuterPayload only if Construct succeeded
			if err == nil {
				innerPayload, err := DeconstructOuterPayload(outerBlob)
				if (err != nil) != tt.expectedErr {
					t.Errorf("DeconstructOuterPayload() error = %v, expectedErr %v", err, tt.expectedErr)
				}
				if err == nil && len(innerPayload) == 0 {
					t.Errorf("DeconstructOuterPayload() returned an empty inner payload")
				}
			}
		})
	}
}
