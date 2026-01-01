package random

import (
	"testing"
)

func TestFourByte(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		checkRange  bool
	}{
		{
			name:        "Valid random value within range",
			expectError: false,
			checkRange:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := FourByte()

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.checkRange {
				if val < 0 || val > 4294967295 {
					t.Errorf("Value out of range: %d", val)
				}
			}
		})
	}
}

func TestPopulateEmptySlice(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectedSize  int
		expectedError bool
	}{
		{
			name:          "Nil slice",
			input:         nil,
			expectedSize:  32, // Expect size to be 32 after population
			expectedError: false,
		},
		{
			name:          "Empty slice",
			input:         []byte{},
			expectedSize:  32, // Expect size to be 32 after population
			expectedError: false,
		},
		{
			name:          "Slice with all zeroes",
			input:         make([]byte, 32),
			expectedSize:  32, // Expect size to be 32 and random data filled
			expectedError: false,
		},
		{
			name:          "Slice with all identical values",
			input:         make([]byte, 32),
			expectedSize:  32, // Expect size to be 32 and random data filled
			expectedError: false,
		},
		{
			name:          "Valid slice",
			input:         []byte{0x01, 0x02, 0x03, 0x04},
			expectedSize:  4, // Should remain unchanged
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the input slice
			inputCopy := append([]byte(nil), tt.input...)

			// Prepare the slice pointer
			slice := &inputCopy

			// Run Test
			err := PopulateEmptySlice(slice, tt.expectedSize)

			// Check for errors
			if tt.expectedError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check if the size of the slice matches the expected size
			if len(*slice) != tt.expectedSize {
				t.Errorf("expected slice size %d, got %d", tt.expectedSize, len(*slice))
			}

			// Additional checks for specific cases
			if len(*slice) == 32 { // Only check the random data cases
				// Check that the slice has been populated with random data
				if isAllZero(*slice) || isAllIdentical(*slice) {
					t.Errorf("slice should not have insecure patterns")
				}
			}
		})
	}
}

func TestNumberInRange(t *testing.T) {
	tests := []struct {
		name      string
		min, max  int
		wantErr   bool
		wantEqual bool // true if we expect the result to equal min (for min == max)
	}{
		{
			name:      "valid range",
			min:       10,
			max:       20,
			wantErr:   false,
			wantEqual: false,
		},
		{
			name:      "min equals max",
			min:       5,
			max:       5,
			wantErr:   false,
			wantEqual: true,
		},
		{
			name:      "min greater than max",
			min:       10,
			max:       5,
			wantErr:   true,
			wantEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NumberInRange(tt.min, tt.max)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return // Stopping here, expecting error
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// When min == max, result must equal min
			if tt.wantEqual && n != tt.min {
				t.Fatalf("expected %d, got %d", tt.min, n)
			}

			// Ensure result in range
			if !tt.wantEqual && (n < tt.min || n > tt.max) {
				t.Fatalf("number out of range: got %d, want between %d and %d", n, tt.min, tt.max)
			}
		})
	}

	// Safety test to check randomness (outside the main table)
	t.Run("randomness check", func(t *testing.T) {
		min, max := 1, 100
		results := make(map[int]bool)
		for i := 0; i < 50; i++ {
			n, err := NumberInRange(min, max)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			results[n] = true
		}
		if len(results) < 2 {
			t.Fatalf("expected multiple distinct results, got only %d unique values", len(results))
		}
	})
}
