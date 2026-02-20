package hmac

import (
	"bytes"
	"testing"
)

func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		name        string
		key         []byte
		data        []byte
		size        int
		wantLen     int
		expectEqual bool
	}{
		{
			name:    "normal input",
			key:     []byte("key"),
			data:    []byte("data"),
			size:    16,
			wantLen: 16,
		},
		{
			name:    "full size",
			key:     []byte("key"),
			data:    []byte("data"),
			size:    32,
			wantLen: 32,
		},
		{
			name:    "empty data",
			key:     []byte("key"),
			data:    nil,
			size:    16,
			wantLen: 16,
		},
		{
			name:    "empty key",
			key:     nil,
			data:    []byte("data"),
			size:    16,
			wantLen: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac1 := ComputeSHA256(tt.key, tt.size, tt.data)
			mac2 := ComputeSHA256(tt.key, tt.size, tt.data)

			if len(mac1) != tt.wantLen {
				t.Fatalf("mac length mismatch: got=%d want=%d", len(mac1), tt.wantLen)
			}

			// determinism check
			if !bytes.Equal(mac1, mac2) {
				t.Fatalf("ComputeSHA256 not deterministic")
			}
		})
	}
}

func TestVerifySHA256(t *testing.T) {
	tests := []struct {
		name      string
		key       []byte
		data      []byte
		size      int
		modifyMac func([]byte) []byte
		wantValid bool
	}{
		{
			name:      "valid mac",
			key:       []byte("key"),
			data:      []byte("data"),
			size:      16,
			wantValid: true,
		},
		{
			name: "wrong key",
			key:  []byte("key"),
			data: []byte("data"),
			size: 16,
			modifyMac: func(mac []byte) []byte {
				return ComputeSHA256([]byte("other-key"), 16, []byte("data"))
			},
			wantValid: false,
		},
		{
			name: "wrong data",
			key:  []byte("key"),
			data: []byte("data"),
			size: 16,
			modifyMac: func(mac []byte) []byte {
				return ComputeSHA256([]byte("key"), 16, []byte("other"))
			},
			wantValid: false,
		},
		{
			name: "wrong size",
			key:  []byte("key"),
			data: []byte("data"),
			size: 16,
			modifyMac: func(mac []byte) []byte {
				return mac[:8]
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac := ComputeSHA256(tt.key, tt.size, tt.data)
			if tt.modifyMac != nil {
				mac = tt.modifyMac(mac)
			}

			valid := VerifySHA256(tt.key, tt.size, tt.data, mac)
			if valid != tt.wantValid {
				t.Fatalf("VerifySHA256 mismatch: got=%v want=%v", valid, tt.wantValid)
			}
		})
	}
}

func TestSHA256_E2E(t *testing.T) {
	tests := []struct {
		name       string
		key        []byte
		data       []byte
		size       int
		verifyKey  []byte
		verifyData []byte
		verifySize int
		wantValid  bool
	}{
		{
			name:       "happy path",
			key:        []byte("key"),
			data:       []byte("data"),
			size:       16,
			verifyKey:  []byte("key"),
			verifyData: []byte("data"),
			verifySize: 16,
			wantValid:  true,
		},
		{
			name:       "size mismatch",
			key:        []byte("key"),
			data:       []byte("data"),
			size:       32,
			verifyKey:  []byte("key"),
			verifyData: []byte("data"),
			verifySize: 16,
			wantValid:  false,
		},
		{
			name:       "empty inputs",
			key:        nil,
			data:       nil,
			size:       16,
			verifyKey:  nil,
			verifyData: nil,
			verifySize: 16,
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac := ComputeSHA256(tt.key, tt.size, tt.data)
			valid := VerifySHA256(tt.verifyKey, tt.verifySize, tt.verifyData, mac)

			if valid != tt.wantValid {
				t.Fatalf("e2e verification mismatch: got=%v want=%v", valid, tt.wantValid)
			}
		})
	}
}
