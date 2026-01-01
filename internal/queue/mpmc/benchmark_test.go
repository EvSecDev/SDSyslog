package mpmc

import (
	"context"
	"fmt"
	"sdsyslog/internal/global"
	"testing"
)

func BenchmarkQueue_Scaling(b *testing.B) {
	// If you are here to find why the overall test is slow, its not
	// Requires a few million iterations to get a stable per-op value (1-2 seconds per size on fast-ish consumer systems)

	sizes := []int{1024, 16384, 131072, 1048576}

	perOp := make([]float64, len(sizes))

	for idx, n := range sizes {
		queue, err := New[int]([]string{global.NSTest}, uint64(n*2), 2, global.DefaultMaxQueueSize)
		if err != nil {
			b.Fatalf("expected no error in creating queue, but got '%v'", err)
		}

		// Warm-up to stabilize caches, allocator, CPU frequency, ect
		for i := 0; i < 1000; i++ {
			queue.Push(i)
			queue.Pop(context.Background())
		}

		b.Run(fmt.Sprintf("QueueCapacity=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				queue.Push(i)
				queue.Pop(context.Background())
			}
			perOp[idx] = float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		})
	}

	// per-op time should be stable as we scale up
	for i := 1; i < len(perOp); i++ {
		if perOp[i] > perOp[i-1]*2.0 {
			b.Fatalf("scaling regression: per-op cost jumped %.2fx (%.2f - %.2f ns/op)",
				perOp[i]/perOp[i-1], perOp[i-1], perOp[i])
		}
	}
}

func BenchmarkQueue_PushAllocations(b *testing.B) {
	queue, err := New[int]([]string{global.NSTest}, 4, 2, global.DefaultMaxQueueSize)
	if err != nil {
		b.Fatalf("expected no error in creating queue, but got '%v'", err)
	}

	allocs := testing.AllocsPerRun(10000, func() {
		queue.Push(42)
		queue.Pop(context.Background())
	})

	if allocs != 0 {
		b.Fatalf("Expected 0 allocations, got %f", allocs)
	}
}
