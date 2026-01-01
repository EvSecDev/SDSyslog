package mpmc

import (
	"context"
	"sdsyslog/internal/global"
	"sync"
	"testing"
	"time"
)

func TestQueueMigration(t *testing.T) {
	tests := []struct {
		name         string
		initialSize  uint64
		newSize      uint64
		numProducers int
		numConsumers int
		numItems     int
	}{
		{
			name:         "scale_up",
			initialSize:  4,
			newSize:      8,
			numProducers: 2,
			numConsumers: 2,
			numItems:     1000,
		},
		{
			name:         "scale_down",
			initialSize:  8,
			newSize:      4,
			numProducers: 2,
			numConsumers: 2,
			numItems:     1000,
		},
		{
			name:         "scale_up_large",
			initialSize:  128,
			newSize:      256,
			numProducers: 8,
			numConsumers: 8,
			numItems:     1000,
		},
		{
			name:         "no_change",
			initialSize:  4,
			newSize:      4,
			numProducers: 2,
			numConsumers: 2,
			numItems:     1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := New[int]([]string{global.NSTest}, tt.initialSize, 2, global.DefaultMaxQueueSize) // min/max not used here
			if err != nil {
				t.Fatalf("failed to create queue: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var wg sync.WaitGroup
			produced := make(chan int, tt.numItems)
			consumed := make(chan int, tt.numItems)

			// Producers
			for p := 0; p < tt.numProducers; p++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for i := id; i < tt.numItems; i += tt.numProducers {
						for !q.Push(i) {
							time.Sleep(time.Microsecond) // backoff
						}
						produced <- i
					}
				}(p)
			}

			// Consumers
			for c := 0; c < tt.numConsumers; c++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for {
						select {
						case <-ctx.Done():
							return
						default:
							if item, ok := q.Pop(ctx); ok {
								consumed <- item
							} else {
								time.Sleep(time.Microsecond)
							}
						}
					}
				}()
			}

			// Trigger resize after some items have been pushed
			time.Sleep(10 * time.Millisecond)
			if err := q.mutateSize(tt.newSize); err != nil {
				t.Fatalf("failed to mutate size: %v", err)
			}
			time.Sleep(10 * time.Millisecond)

			// Stop test readers/writers
			cancel()

			// Wait for producers to finish
			wg.Wait()

			// Close channels to finish processing
			close(consumed)
			close(produced)

			// Verify all items produced were consumed
			producedMap := make(map[int]struct{})
			producedCount := 0
			for v := range produced {
				producedMap[v] = struct{}{}
				producedCount++
			}

			consumedCount := 0
			for v := range consumed {
				consumedCount++
				if _, ok := producedMap[v]; !ok {
					t.Errorf("consumed unknown item: %d", v)
				} else {
					delete(producedMap, v)
				}
			}

			if len(producedMap) != 0 {
				t.Errorf("items not consumed: %v", producedMap)
			}

			// Final check, produced vs consumed count
			if producedCount != consumedCount {
				t.Errorf("produced count %d != consumed count %d", producedCount, consumedCount)
			}
		})
	}
}
