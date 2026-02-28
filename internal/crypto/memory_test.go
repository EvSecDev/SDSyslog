package crypto

import (
	"testing"
	"unsafe"
)

func TestMemzero(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "Nil input",
			input: nil,
		},
		{
			name:  "Empty Item",
			input: []byte{},
		},
		{
			name:  "Single Item",
			input: []byte{1},
		},
		{
			name:  "Multiple Items",
			input: []byte{1, 2, 3, 4, 5},
		},
		{
			name:  "Zeroes Items",
			input: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			name:  "Large items",
			input: make([]byte, 1024), // 1KB
		},
		{
			name:  "Larger Slice",
			input: make([]byte, 100*1024), // 100KB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("expected no panic, but got a panic: %v\n", r)
				}
			}()

			var addressBefore uintptr
			if tt.input != nil {
				// Grab input address before zero
				addressBefore = uintptr(unsafe.Pointer(&tt.input))
			}

			// Zero the slice
			Memzero(tt.input)

			if tt.input == nil {
				// No further verifications for nil inputs
				return
			}

			// Verify the slice is zero
			for _, value := range tt.input {
				if value != 0 {
					t.Errorf("expected 0, got %d", value)
				}
			}

			// Verify slice is not nil
			if tt.input == nil {
				t.Errorf("expected slice to be non-nil, got nil")
			}

			// Verify memory address after has not changed
			addressAfter := uintptr(unsafe.Pointer(&tt.input))
			if addressBefore != addressAfter {
				t.Errorf("expected memory address to remain the same, but got different addresses: before=%x, after=%x", addressBefore, addressAfter)
			}

			// Use unsafe to verify the contents of the slice have been zeroed in memory
			for i := 0; i < len(tt.input); i++ {
				if *(*byte)(unsafe.Pointer(&tt.input[i])) != 0 {
					t.Errorf("memory at index %d was not zeroed, expected 0, got %d", i, tt.input[i])
				}
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  bool
	}{
		{
			name:  "nil slice",
			input: nil,
			want:  true,
		},
		{
			name:  "empty slice",
			input: []byte{},
			want:  true,
		},
		{
			name:  "all zeros small",
			input: []byte{0, 0, 0},
			want:  true,
		},
		{
			name:  "all zeros large",
			input: make([]byte, 1024),
			want:  true,
		},
		{
			name:  "single non-zero at start",
			input: []byte{1, 0, 0},
			want:  false,
		},
		{
			name:  "single non-zero in middle",
			input: []byte{0, 2, 0},
			want:  false,
		},
		{
			name:  "single non-zero at end",
			input: []byte{0, 0, 3},
			want:  false,
		},
		{
			name:  "all non-zero",
			input: []byte{1, 2, 3},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsZero(tt.input)
			if got != tt.want {
				t.Fatalf("input=%v, got %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
