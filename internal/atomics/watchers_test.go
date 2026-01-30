package atomics

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitUntilZero(t *testing.T) {
	tests := []struct {
		name          string
		initial       uint64
		mutate        func(a *atomic.Uint64)
		maxWaitTime   time.Duration
		expectReached bool
	}{
		{
			name:    "already zero",
			initial: 0,
			mutate: func(a *atomic.Uint64) {
				// no-op
			},
			maxWaitTime:   200 * time.Millisecond,
			expectReached: true,
		},
		{
			name:    "eventually reaches zero",
			initial: 5,
			mutate: func(a *atomic.Uint64) {
				go func() {
					time.Sleep(100 * time.Millisecond)
					a.Store(0)
				}()
			},
			maxWaitTime:   500 * time.Millisecond,
			expectReached: true,
		},
		{
			name:    "never reaches zero",
			initial: 3,
			mutate: func(a *atomic.Uint64) {
				// no-op
			},
			maxWaitTime:   200 * time.Millisecond,
			expectReached: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a atomic.Uint64
			a.Store(tt.initial)

			tt.mutate(&a)

			reached, last := WaitUntilZero(&a, tt.maxWaitTime)

			if reached != tt.expectReached {
				t.Fatalf("expected reached=%v, got %v (last=%d)",
					tt.expectReached, reached, last)
			}

			if reached && last != 0 {
				t.Fatalf("expected last value to be 0, got %d", last)
			}
		})
	}
}
