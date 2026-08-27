package mpmc

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
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
			queue, err := New[int]([]string{logctx.NSTest}, tt.initialSize, 2, global.DefaultMaxQueueSize) // min/max not used here
			if err != nil {
				t.Fatalf("failed to create queue: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var wg sync.WaitGroup
			produced := make(chan int, tt.numItems)
			consumed := make(chan int, tt.numItems)

			// Producers
			for producerNum := range tt.numProducers {
				wg.Go(func() {
					for index := producerNum; index < tt.numItems; index += tt.numProducers {
						for {
							select {
							case <-ctx.Done():
								return
							default:
							}
							err := queue.Push(index, 1)
							if err == nil {
								break
							}
							time.Sleep(time.Microsecond) // backoff
						}
						produced <- index
					}
				})
			}

			// Consumers
			for range tt.numConsumers {
				wg.Go(func() {
					for {
						select {
						case <-ctx.Done():
							return
						default:
							if item, ok := queue.Pop(ctx); ok {
								consumed <- item
							} else {
								time.Sleep(time.Microsecond)
							}
						}
					}
				})
			}

			// Trigger resize after some items have been pushed
			time.Sleep(10 * time.Millisecond)
			err = queue.mutateSize(tt.newSize)
			if err != nil {
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
			for producedItem := range produced {
				producedMap[producedItem] = struct{}{}
				producedCount++
			}

			consumedCount := 0
			for index := range consumed {
				consumedCount++
				if _, ok := producedMap[index]; !ok {
					t.Errorf("consumed unknown item: %d", index)
				} else {
					delete(producedMap, index)
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

func TestQueueMigration_EmptyAtMutation(t *testing.T) {
	tests := []struct {
		name string
		// Consumer blocked before the mutation waits on the old migration channel,
		// after the mutation it waits on the new one
		consumerBeforeMutation bool
	}{
		{
			name:                   "consumer_blocked_before_mutation",
			consumerBeforeMutation: true,
		},
		{
			name:                   "consumer_starts_after_mutation",
			consumerBeforeMutation: false,
		},
	}

	const (
		initialSize = 4
		newSize     = 8
		numItems    = 100
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue, err := New[int]([]string{logctx.NSTest}, initialSize, 2, global.DefaultMaxQueueSize) // min/max not used here
			if err != nil {
				t.Fatalf("failed to create queue: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Prime and drain the initial queue so it is deterministically empty at mutation time
			for range initialSize {
				err := queue.Push(0, 1)
				if err != nil {
					t.Fatalf("failed to prime queue: %v", err)
				}
				_, ok := queue.Pop(ctx)
				if !ok {
					t.Fatalf("failed to drain queue")
				}
			}

			var wg sync.WaitGroup
			produced := make(chan int, numItems)
			consumed := make(chan int, numItems)

			startConsumer := func() {
				wg.Go(func() {
					for {
						select {
						case <-ctx.Done():
							return
						default:
							item, ok := queue.Pop(ctx)
							if ok {
								consumed <- item
							}
						}
					}
				})
			}

			if tt.consumerBeforeMutation {
				startConsumer()

				// Wait until the consumer is provably blocked in the old queue's empty wait
				deadline := time.Now().Add(2 * time.Second)
				for queue.ActiveRead.Load().Metrics.PopEmptySeqBehind.Load() == 0 {
					if time.Now().After(deadline) {
						t.Fatalf("consumer did not block on the old queue before mutation")
					}
					time.Sleep(time.Millisecond)
				}
			}

			err = queue.mutateSize(newSize)
			if err != nil {
				t.Fatalf("failed to mutate size: %v", err)
			}

			if !tt.consumerBeforeMutation {
				startConsumer()
			}

			wg.Go(func() {
				for item := range numItems {
					for {
						select {
						case <-ctx.Done():
							return
						default:
						}
						err := queue.Push(item, 1)
						if err == nil {
							break
						}
						time.Sleep(time.Millisecond) // backoff
					}
					produced <- item
				}
			})

			// If the consumer never migrates, the new queue fills up and this stalls
			consumedItems := make([]int, 0, numItems)
			timeout := time.NewTimer(2 * time.Second)
			defer timeout.Stop()
			for len(consumedItems) < numItems {
				select {
				case item := <-consumed:
					consumedItems = append(consumedItems, item)
				case <-timeout.C:
					t.Fatalf("consumer did not migrate to new queue: only %d of %d items consumed",
						len(consumedItems), numItems)
				}
			}

			// Migration is complete when the read pointer follows the write pointer
			if queue.ActiveRead.Load() != queue.ActiveWrite.Load() {
				t.Errorf("consumer never completed migration to the new queue")
			}

			// Stop test readers/writers
			cancel()
			wg.Wait()

			// Verify all items produced were consumed
			close(produced)

			producedMap := make(map[int]struct{})
			for producedItem := range produced {
				producedMap[producedItem] = struct{}{}
			}

			for index := range consumedItems {
				_, ok := producedMap[index]
				if !ok {
					t.Errorf("consumed unknown item: %d", index)
				} else {
					delete(producedMap, index)
				}
			}

			if len(producedMap) != 0 {
				t.Errorf("items not consumed: %v", producedMap)
			}
		})
	}
}
