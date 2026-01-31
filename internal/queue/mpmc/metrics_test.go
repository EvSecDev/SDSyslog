package mpmc

import (
	"context"
	"testing"
	"time"
)

func TestMetricsPresence(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		initialCap    uint64
		minCap        int
		maxCap        int
		pushCount     int
		bytesPerItem  int
		expectDepth   uint64
		expectBytes   uint64
		expectPush    uint64
		expectPushSuc uint64
		expectPop     uint64
		expectPopSuc  uint64
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
				success := q.Push(i)
				if !success {
					t.Fatalf("push %d failed unexpectedly", i)
				}
				// simulate bytes metric
				q.ActiveWrite.Load().Metrics.Bytes.Add(uint64(tt.bytesPerItem))
			}

			// Pop items (consume some/all)
			popCount := tt.expectPop
			for i := 0; i < int(popCount); i++ {
				val, success := q.Pop(ctx)
				if !success {
					t.Fatalf("pop %d failed unexpectedly", i)
				}
				_ = val
				// decrement bytes to simulate consumption
				q.ActiveRead.Load().Metrics.Bytes.Add(^uint64(tt.bytesPerItem - 1)) // subtract
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

			if got := mmap["depth"]; got != tt.expectDepth {
				t.Errorf("depth: got %d, want %d", got, tt.expectDepth)
			}
			if got := mmap["byte_sum"]; got != tt.expectBytes {
				t.Errorf("byte_sum: got %d, want %d", got, tt.expectBytes)
			}
			if got := mmap["push_attempts"]; got != tt.expectPush {
				t.Errorf("push_attempts: got %d, want %d", got, tt.expectPush)
			}
			if got := mmap["push_success"]; got != tt.expectPushSuc {
				t.Errorf("push_success: got %d, want %d", got, tt.expectPushSuc)
			}
			if got := mmap["pop_attempts"]; got != tt.expectPop {
				t.Errorf("pop_attempts: got %d, want %d", got, tt.expectPop)
			}
			if got := mmap["pop_success"]; got != tt.expectPopSuc {
				t.Errorf("pop_success: got %d, want %d", got, tt.expectPopSuc)
			}
		})
	}
}
