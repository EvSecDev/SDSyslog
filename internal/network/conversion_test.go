package network

import (
	"testing"
)

func TestIPtoIntegers(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expectedHi  int
		expectedLo  int
		expectedErr bool
	}{
		{
			name:        "valid ipv4",
			input:       "192.168.0.1",
			expectedHi:  3232235521,
			expectedLo:  0,
			expectedErr: false,
		},
		{
			name:        "invalid ipv4",
			input:       "192.168a.0.1",
			expectedHi:  0,
			expectedLo:  0,
			expectedErr: true,
		},
		{
			name:        "valid ipv6 full",
			input:       "fd01:1:22:def:060:f41:436a:353",
			expectedHi:  215891302839874065,
			expectedLo:  27038370742534995,
			expectedErr: false,
		},
		{
			name:        "valid ipv6 short",
			input:       "fd01::060:f41:436a:353",
			expectedHi:  215891307137073152,
			expectedLo:  27038370742534995,
			expectedErr: false,
		},
		{
			name:        "invalid ipv6 short",
			input:       "fd01::060z:f41:436a:353",
			expectedHi:  0,
			expectedLo:  0,
			expectedErr: true,
		},
		{
			name:        "invalid ipv6 full",
			input:       "fd01:1:22:def:060:f41:436a:353:eee",
			expectedHi:  0,
			expectedLo:  0,
			expectedErr: true,
		},
		{
			name:        "valid ipv6 full bracketed",
			input:       "[fd01:1:22:def:060:f41:436a:353]",
			expectedHi:  0,
			expectedLo:  0,
			expectedErr: true,
		},
		{
			name:        "empty",
			input:       "",
			expectedHi:  0,
			expectedLo:  0,
			expectedErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			hi, lo, err := IPtoIntegers(tt.input)
			if tt.expectedErr && err == nil {
				t.Fatalf("expected error, but got no error")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("expected no error, but got '%v'", err)
			}

			if tt.expectedHi != hi {
				t.Errorf("expected high integer to be '%v' but got '%v'", tt.expectedHi, hi)
			}
			if tt.expectedLo != lo {
				t.Errorf("expected low integer to be '%v' but got '%v'", tt.expectedLo, lo)
			}
		})
	}
}
