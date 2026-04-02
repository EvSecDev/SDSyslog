package protocol

import (
	"bytes"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/tests/utils"
	"sdsyslog/pkg/crypto/registry"
	"strings"
	"testing"
)

func TestConstructAndDeconstructouter(t *testing.T) {
	tests := []struct {
		name                   string
		suiteID                uint8
		pubKey                 []byte
		payload                []byte
		expectedConstructErr   string
		expectedDeconstructErr string
	}{
		{
			name:    "Valid Input",
			suiteID: 1,
			pubKey:  []byte("placeholder"),
			payload: []byte(strings.Repeat("a", minInnerPayloadLen)),
		},
		{
			name:                 "Empty Payload",
			suiteID:              1,
			pubKey:               []byte{0x01, 0x02},
			payload:              nil,
			expectedConstructErr: "protocol violation: payload cannot be empty",
		},
		{
			name:                 "Invalid Key Length",
			suiteID:              1,
			pubKey:               []byte{0x01},
			payload:              []byte("some data"),
			expectedConstructErr: "invalid public key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, validID := registry.GetSuiteInfo(tt.suiteID)
			if !validID {
				t.Fatalf("invalid suite ID %d", tt.suiteID)
			}
			priv, pub, err := info.NewKey()
			if err != nil {
				t.Fatalf("failed to generate keys: %v", err)
			}
			if bytes.Equal(tt.pubKey, []byte("placeholder")) {
				tt.pubKey = pub
			}

			err = wrappers.SetupEncryptInnerPayload(tt.pubKey)
			if err != nil {
				t.Fatalf("unexpected error setting up encryption function: %v", err)
			}
			err = wrappers.SetupDecryptInnerPayload(priv)
			if err != nil {
				t.Fatalf("unexpected error setting up decryption function: %v", err)
			}

			// Test ConstructOuterPayload
			outerBlob, err := ConstructOuterPayload(tt.payload, tt.suiteID)
			matches, err := utils.MatchErrorString(err, tt.expectedConstructErr)
			if err != nil {
				t.Fatalf("%v", err)
			} else if matches {
				return
			}

			// Test DeconstructOuterPayload only if Construct succeeded
			innerPayload, err := DeconstructOuterPayload(outerBlob)
			matches, err = utils.MatchErrorString(err, tt.expectedConstructErr)
			if err != nil {
				t.Fatalf("%v", err)
			} else if matches {
				return
			}
			if !bytes.Equal(innerPayload, tt.payload) {
				t.Fatalf("expected payload %q, but got %q", string(tt.payload), string(innerPayload))
			}
		})
	}
}
