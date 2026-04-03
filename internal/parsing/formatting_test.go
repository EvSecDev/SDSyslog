package parsing

import (
	"sdsyslog/internal/tests/utils"
	"testing"
	"time"
)

func TestTrimDurationPrecision(t *testing.T) {
	tests := []struct {
		name         string
		input        time.Duration
		wantDecimals int
		expected     string
	}{
		{
			name:         "empty",
			wantDecimals: 2,
			expected:     "0s",
		},
		{
			name:         "hours",
			input:        319 * time.Minute,
			wantDecimals: 2,
			expected:     "5h19m0s",
		},
		{
			name:         "minutes",
			input:        132 * time.Second,
			wantDecimals: 2,
			expected:     "2m12s",
		},
		{
			name:         "seconds",
			input:        1543 * time.Millisecond,
			wantDecimals: 2,
			expected:     "1.54s",
		},
		{
			name:         "milliseconds",
			input:        2431 * time.Microsecond,
			wantDecimals: 3,
			expected:     "2.431ms",
		},
		{
			name:         "microseconds",
			input:        3493 * time.Nanosecond,
			wantDecimals: 1,
			expected:     "3.4µs",
		},
		{
			name:         "nanoseconds",
			input:        2 * time.Nanosecond,
			wantDecimals: 2,
			expected:     "2ns",
		},
		{
			name:         "fraction shorter than precision",
			input:        1500 * time.Millisecond,
			wantDecimals: 3,
			expected:     "1.5s",
		},
		{
			name:         "fraction exactly precision",
			input:        1540 * time.Millisecond,
			wantDecimals: 2,
			expected:     "1.54s",
		},
		{
			name:         "trim long fraction",
			input:        1234567890 * time.Nanosecond,
			wantDecimals: 3,
			expected:     "1.234s",
		},
		{
			name:         "mixed units",
			input:        2*time.Hour + 3*time.Minute + 456789000*time.Nanosecond,
			wantDecimals: 2,
			expected:     "2h3m0.45s",
		},
		{
			name:         "mixed units no fraction",
			input:        2*time.Hour + 3*time.Minute + 4*time.Second,
			wantDecimals: 2,
			expected:     "2h3m4s",
		},
		{
			name:         "zero precision",
			input:        1543 * time.Millisecond,
			wantDecimals: 0,
			expected:     "1s",
		},
		{
			name:         "negative duration",
			input:        -1543 * time.Millisecond,
			wantDecimals: 2,
			expected:     "-1.54s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := TrimDurationPrecision(tt.input, tt.wantDecimals)
			if output != tt.expected {
				t.Fatalf("expected output %q, but got %q", tt.expected, output)
			}
		})
	}
}

func TestVerifyWholeDuration(t *testing.T) {
	tests := []struct {
		name          string
		input         time.Duration
		expectedError string
	}{
		{
			name:  "clean ms",
			input: 50 * time.Millisecond,
		},
		{
			name:  "clean ms 2",
			input: 200 * time.Millisecond,
		},
		{
			name:  "clean second",
			input: 5 * time.Second,
		},
		{
			name:  "clean second 2",
			input: 15 * time.Second,
		},
		{
			name:  "clean minute",
			input: 1 * time.Minute,
		},
		{
			name:          "too large",
			input:         1 * time.Hour,
			expectedError: " must divide evenly into ",
		},
		{
			name:          "negative",
			input:         -1 * time.Second,
			expectedError: "interval must be positive",
		},
		{
			name:          "second not divisible",
			input:         23 * time.Second,
			expectedError: " must divide evenly into ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyWholeDuration(tt.input)
			_, err = utils.MatchErrorString(err, tt.expectedError)
			if err != nil {
				t.Fatalf("%v", err)
			}
		})
	}
}
