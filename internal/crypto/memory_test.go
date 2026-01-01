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
