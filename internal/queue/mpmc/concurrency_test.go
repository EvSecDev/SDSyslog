package mpmc

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sdsyslog/internal/global"
	"sync"
	"testing"
	"time"
)

func TestQueue_Concurrency(t *testing.T) {
	tests := []struct {
		name          string
		capacity      uint64
		numGoroutines int
		numOps        int
	}{
		{"ConcurrentSmallQueue", 128, 1, 100},
		{"HighContention", 16, 10, 1000},
		{"ThreadSafetyLargeQueue", 1024, 1, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue, err := New[int]([]string{global.NSTest}, tt.capacity, 2, global.DefaultMaxQueueSize)
			if err != nil {
				t.Fatalf("expected no error in creating queue, but got '%v'", err)
			}

			done := make(chan bool, tt.numGoroutines*2)

			for i := 0; i < tt.numGoroutines; i++ {
				go func() {
					for j := 0; j < tt.numOps; j++ {
						for !queue.Push(j) {
							runtime.Gosched()
						}
					}
					done <- true
				}()
				go func() {
					for j := 0; j < tt.numOps; j++ {
						_, success := queue.Pop(context.Background())
						if !success {
							t.Errorf("Pop failed during high contention")
						}
					}
					done <- true
				}()
			}

			for i := 0; i < tt.numGoroutines*2; i++ {
				<-done
			}
		})
	}
}

func TestQueue_ContextBehavior(t *testing.T) {
	t.Run("PopBlocksUntilPush", func(t *testing.T) {
		queue, err := New[int]([]string{global.NSTest}, 2, 2, global.DefaultMaxQueueSize)
		if err != nil {
			t.Fatalf("expected no error in creating queue, but got '%v'", err)
		}

		done := make(chan int)
		go func() {
			result, success := queue.Pop(context.Background())
			if !success || result != 42 {
				t.Errorf("Expected pop to return 42, got %v", result)
			}
			done <- result
		}()
		time.Sleep(50 * time.Millisecond)
		queue.Push(42)
		<-done
	})

	t.Run("PopTimeout", func(t *testing.T) {
		queue, err := New[int]([]string{global.NSTest}, 2, 2, global.DefaultMaxQueueSize)
		if err != nil {
			t.Fatalf("expected no error in creating queue, but got '%v'", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, success := queue.Pop(ctx)
		if success {
			t.Fatalf("Expected pop to fail due to timeout")
		}
	})

	t.Run("PopContextCancel", func(t *testing.T) {
		queue, err := New[int]([]string{global.NSTest}, 2, 2, global.DefaultMaxQueueSize)
		if err != nil {
			t.Fatalf("expected no error in creating queue, but got '%v'", err)
		}

		queue.Push(10)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, success := queue.Pop(ctx)
		if !success {
			t.Fatalf("Expected pop to succeed when context cancelled after push")
		}
	})
}

func TestQueue_StressIntegrity(t *testing.T) {
	// Create a queue instance with a given namespace and size
	queue, err := New[int]([]string{global.NSTest}, 512, 2, global.DefaultMaxQueueSize)
	if err != nil {
		t.Fatalf("expected no error in creating queue, but got '%v'", err)
	}

	// Test configuration
	const N = 20000
	const numProducers = 4
	const numConsumers = 4

	// Trackers for produced/consumed values success
	var produced sync.Map
	var consumed sync.Map

	errCh := make(chan error, numProducers+numConsumers) // To capture errors from goroutines
	var wg sync.WaitGroup
	wg.Add(numProducers + numConsumers) // Adding goroutines

	// Helper function for producers
	producer := func(id int) {
		defer wg.Done()
		for i := 0; i < N/numProducers; i++ {
			for !queue.Push(i + id*N/numProducers) { // Push different range per producer
				// Random delay between push attempts
				time.Sleep(time.Nanosecond)
			}
			produced.Store(i+id*N/numProducers, true) // Mark value as produced
		}
	}

	// Helper function for consumers
	consumer := func() {
		defer wg.Done()
		for i := 0; i < N/numConsumers; i++ {
			// Randomize sleep interval to simulate varied workloads
			time.Sleep(time.Duration(rand.Intn(50)) * time.Microsecond)

			v, ok := queue.Pop(context.Background())
			if !ok {
				errCh <- fmt.Errorf("pop failed at iteration %d", i)
				return
			}
			if v < 0 || v >= N {
				errCh <- fmt.Errorf("invalid value popped: %d", v)
				return
			}
			consumed.Store(v, true) // Mark value as consumed
		}
	}

	// Start multiple producers
	for i := 0; i < numProducers; i++ {
		go producer(i)
	}

	// Start multiple consumers
	for i := 0; i < numConsumers; i++ {
		go consumer()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errCh)

	// Handle any errors from the goroutines
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	// Validate integrity after all the work is done
	// Ensure all produced values were consumed
	for i := 0; i < N; i++ {
		// Check if each produced value has been consumed
		_, producedOk := produced.Load(i)
		_, consumedOk := consumed.Load(i)

		if !producedOk {
			t.Errorf("value %d was never produced", i)
		}
		if !consumedOk {
			t.Errorf("value %d was never consumed", i)
		}
	}
}
