package mpmc

import (
	"context"
	"sdsyslog/internal/global"
	"testing"
	"time"
)

func TestMetricsPresence(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		initialCap   uint64
		minCap       global.MinValue
		maxCap       global.MaxValue
		pushCount    int
		bytesPerItem int
		expectDepth  uint64
		expectBytes  uint64

		expectPush          uint64
		expectPushSuc       uint64
		expectPushCAS       uint64
		expectPushSeqBehind uint64
		expectPushStale     uint64

		expectPop         uint64
		expectPopSuc      uint64
		expectPopCAS      uint64
		expectPopEmptySeq uint64
		expectPopStale    uint64
	}{
		{
			name:          "simple producer-consumer",
			initialCap:    8,
			minCap:        4,
			maxCap:        16,
			pushCount:     5,
			bytesPerItem:  10,
			expectDepth:   0, // consumer will pop all
			expectBytes:   0, // after pops
			expectPush:    5,
			expectPushSuc: 5,
			expectPop:     5,
			expectPopSuc:  5,
		},
		{
			name:          "partial consumption",
			initialCap:    8,
			minCap:        4,
			maxCap:        16,
			pushCount:     3,
			bytesPerItem:  20,
			expectDepth:   1,  // consume 2
			expectBytes:   20, // 1 remaining
			expectPush:    3,
			expectPushSuc: 3,
			expectPop:     2,
			expectPopSuc:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := New[int]([]string{"test"}, tt.initialCap, tt.minCap, tt.maxCap)
			if err != nil {
				t.Fatalf("failed to create queue: %v", err)
			}

			// Push items
			for i := 0; i < tt.pushCount; i++ {
				err = q.Push(i, uint64(tt.bytesPerItem))
				if err != nil {
					t.Fatalf("push %d failed unexpectedly: %v", i, err)
				}
			}

			// Pop items (consume some/all)
			popCount := tt.expectPop
			for i := 0; i < int(popCount); i++ {
				val, success := q.Pop(ctx)
				if !success {
					t.Fatalf("pop %d failed unexpectedly", i)
				}
				_ = val
			}

			// Collect metrics
			metricsCollected := q.CollectMetrics(time.Second)

			// Convert to map for easier assertions
			mmap := map[string]uint64{}
			for _, m := range metricsCollected {
				if v, ok := m.Value.Raw.(uint64); ok {
					mmap[m.Name] = v
				}
			}

			if got := mmap[MTDepth]; got != tt.expectDepth {
				t.Errorf("%s: got %d, want %d", MTDepth, got, tt.expectDepth)
			}
			if got := mmap[MTBytes]; got != tt.expectBytes {
				t.Errorf("%s: got %d, want %d", MTBytes, got, tt.expectBytes)
			}
			if got := mmap[MTPushAttempt]; got != tt.expectPush {
				t.Errorf("%s: got %d, want %d", MTPushAttempt, got, tt.expectPush)
			}
			if got := mmap[MTPushSuc]; got != tt.expectPushSuc {
				t.Errorf("%s: got %d, want %d", MTPushSuc, got, tt.expectPushSuc)
			}
			if got := mmap[MTPushCASRetry]; got != tt.expectPushCAS {
				t.Errorf("%s: got %d, want %d", MTPushCASRetry, got, tt.expectPushCAS)
			}
			if got := mmap[MTPushSeqBehindTail]; got != tt.expectPushSeqBehind {
				t.Errorf("%s: got %d, want %d", MTPushSeqBehindTail, got, tt.expectPushSeqBehind)
			}
			if got := mmap[MTPushStaleRetries]; got != tt.expectPushStale {
				t.Errorf("%s: got %d, want %d", MTPushStaleRetries, got, tt.expectPushStale)
			}

			if got := mmap[MTPopAttempt]; got != tt.expectPop {
				t.Errorf("%s: got %d, want %d", MTPopAttempt, got, tt.expectPop)
			}
			if got := mmap[MTPopSuc]; got != tt.expectPopSuc {
				t.Errorf("%s: got %d, want %d", MTPopSuc, got, tt.expectPopSuc)
			}
			if got := mmap[MTPopCASRetry]; got != tt.expectPopCAS {
				t.Errorf("%s: got %d, want %d", MTPopCASRetry, got, tt.expectPopCAS)
			}
			if got := mmap[MTPopEmptySeq]; got != tt.expectPopEmptySeq {
				t.Errorf("%s: got %d, want %d", MTPopEmptySeq, got, tt.expectPopEmptySeq)
			}
			if got := mmap[MTPopStaleRetries]; got != tt.expectPopStale {
				t.Errorf("%s: got %d, want %d", MTPopStaleRetries, got, tt.expectPopStale)
			}
		})
	}
}
