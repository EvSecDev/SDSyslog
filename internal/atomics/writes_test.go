package atomics

import (
	"sync/atomic"
	"testing"
)

func TestSubtract(t *testing.T) {
	tests := []struct {
		name        string
		initial     uint64
		subtract    uint64
		maxRetries  int
		mutate      func(a *atomic.Uint64)
		wantSuccess bool
		wantFinal   uint64
	}{
		{
			name:        "already zero",
			initial:     0,
			subtract:    5,
			maxRetries:  1,
			wantSuccess: true,
			wantFinal:   0,
		},
		{
			name:        "simple subtraction",
			initial:     10,
			subtract:    3,
			maxRetries:  3,
			wantSuccess: true,
			wantFinal:   7,
		},
		{
			name:        "subtract more than available",
			initial:     5,
			subtract:    10,
			maxRetries:  3,
			wantSuccess: true,
			wantFinal:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a atomic.Uint64
			a.Store(tt.initial)

			if tt.mutate != nil {
				tt.mutate(&a)
			}

			ok := Subtract(&a, tt.subtract, tt.maxRetries)
			final := a.Load()

			if ok != tt.wantSuccess {
				t.Fatalf("expected success=%v, got %v", tt.wantSuccess, ok)
			}

			if final != tt.wantFinal {
				t.Fatalf("expected final=%d, got %d", tt.wantFinal, final)
			}
		})
	}
}
