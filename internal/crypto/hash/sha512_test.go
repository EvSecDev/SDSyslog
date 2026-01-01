package hash

import (
	"bytes"
	"crypto/sha512"
	"testing"
)

func TestMultipleSlices(t *testing.T) {
	testEmptyHash := sha512.Sum512(nil)
	expectedEmptyHash := testEmptyHash[:]

	testDataHash := sha512.Sum512([]byte("Hello, world!"))
	expectedDataHash := testDataHash[:]

	tests := []struct {
		name     string
		input    [][]byte
		expected []byte
	}{
		{
			name:     "Nil Input",
			input:    nil,
			expected: expectedEmptyHash,
		},
		{
			name: "Empty Input",
			input: [][]byte{
				[]byte(""),
			},
			expected: expectedEmptyHash,
		},
		{
			name: "Double Nil Input",
			input: [][]byte{
				nil,
				nil,
			},
			expected: expectedEmptyHash,
		},
		{
			name: "Double Empty Input",
			input: [][]byte{
				[]byte(""),
				[]byte(""),
			},
			expected: expectedEmptyHash,
		},
		{
			name: "Single slice",
			input: [][]byte{
				[]byte("Hello, world!"),
			},
			expected: expectedDataHash,
		},
		{
			name: "Multiple slices",
			input: [][]byte{
				[]byte("Hello, "),
				[]byte("world!"),
			},
			expected: expectedDataHash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := MultipleSlices(tt.input...)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if !bytes.Equal(hash, tt.expected) {
				t.Errorf("expected hash %x, got %x", tt.expected, hash)
			}
		})
	}
}
