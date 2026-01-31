package mpmc

import (
	"context"
	"sdsyslog/internal/global"
	"testing"
	"time"
)

// Helper
func intPtr[T any](v T) *T { return &v }

func TestQueue_PushPopScenarios(t *testing.T) {
	type op struct {
		push *int // nil means pop
		want *int // nil means no expected output
	}

	tests := []struct {
		name     string
		capacity uint64
		ops      []op
	}{
		{
			name:     "SinglePushPop",
			capacity: 32,
			ops: []op{
				{push: intPtr(10)},
				{want: intPtr(10)},
			},
		},
		{
			name:     "SimpleWrap",
			capacity: 4,
			ops: []op{
				{push: intPtr(1)},
				{push: intPtr(2)},
				{push: intPtr(3)},
				{push: intPtr(4)},
				{want: intPtr(1)},
				{want: intPtr(2)},
			},
		},
		{
			name:     "DeepWrap",
			capacity: 4,
			ops: []op{
				{push: intPtr(0)},
				{push: intPtr(1)},
				{push: intPtr(2)},
				{push: intPtr(3)},
				{want: intPtr(0)},
				{want: intPtr(1)},
				{push: intPtr(100)}, // wrap happens here
				{push: intPtr(200)},
				{want: intPtr(2)},
				{want: intPtr(3)},
				{want: intPtr(100)},
				{want: intPtr(200)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := New[int]([]string{global.NSTest}, tt.capacity, 2, global.DefaultMaxQueueSize)
			if err != nil {
				t.Fatalf("expected no error in creating queue, but got '%v'", err)
			}

			for i, op := range tt.ops {
				if op.push != nil {
					if !q.Push(*op.push) {
						t.Fatalf("op %d: push(%d) failed", i, *op.push)
					}
				} else if op.want != nil {
					got, ok := q.Pop(context.Background())
					if !ok {
						t.Fatalf("op %d: pop failed", i)
					}
					if got != *op.want {
						t.Fatalf("op %d: want %d, got %d", i, *op.want, got)
					}
				}
			}
		})
	}
}

func TestNewQueue_InvalidCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity uint64
	}{
		{"Capacity3", 3},
		{"Capacity0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New[int]([]string{global.NSTest}, tt.capacity, 2, global.DefaultMaxQueueSize)
			if err == nil {
				t.Fatalf("expected error in creating queue, but got nil")
			}
		})
	}
}

func TestPushFailures(t *testing.T) {
	tests := []struct {
		name     string
		capacity uint64
		prefill  []int
		testPush int
		expectOK bool
	}{
		{"FullQueueFails", 4, []int{1, 2, 3, 4}, 5, false},
		{"RetryAfterSpace", 2, []int{1, 2}, 3, false}, // first fail
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := New[int]([]string{global.NSTest}, tt.capacity, 2, global.DefaultMaxQueueSize)
			if err != nil {
				t.Fatalf("expected no error in creating queue, but got '%v'", err)
			}

			for _, v := range tt.prefill {
				q.Push(v)
			}

			ok := q.Push(tt.testPush)
			if ok != tt.expectOK {
				t.Fatalf("expected %v, got %v", tt.expectOK, ok)
			}

			// Special case: retry test
			if tt.name == "RetryAfterSpace" {
				q.Pop(context.Background())
				if !q.Push(tt.testPush) {
					t.Fatalf("retry push should succeed")
				}
			}
		})
	}
}

func TestNotEmptyChannel(t *testing.T) {
	queue, err := New[int]([]string{global.NSTest}, 8, 2, global.DefaultMaxQueueSize)
	if err != nil {
		t.Fatalf("expected no error in creating queue, but got '%v'", err)
	}

	// Test that the notEmpty channel works correctly
	go func() {
		for i := 0; i < 5; i++ {
			success := queue.Push(i)
			if !success {
				t.Errorf("Push failed for value %d", i)
			}
		}
	}()

	// Now wait for the queue to have elements and ensure the channel is notified
	qPtr := queue.ActiveRead.Load()
	select {
	case <-qPtr.notEmpty:
		// Test passed, do nothing here
	case <-time.After(1 * time.Second):
		t.Errorf("Timeout waiting for notEmpty channel")
	}
}

func TestQueueThroughput(t *testing.T) {
	queue, err := New[int]([]string{global.NSTest}, 16777216, 2, global.DefaultMaxQueueSize)
	if err != nil {
		t.Fatalf("expected no error in creating queue, but got '%v'", err)
	}

	// Simulate high throughput
	for i := 0; i < 10000000; i++ {
		success := queue.Push(i)
		if !success {
			t.Fatalf("Push failed for value %d", i)
		}
	}

	for i := 0; i < 10000000; i++ {
		_, success := queue.Pop(context.Background())
		if !success {
			t.Fatalf("Pop failed for index %d", i)
		}
	}
}
