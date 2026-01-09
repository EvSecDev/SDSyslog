// Multi-producer Multi-Consumer lock-free ring buffer queue with power-of-two capacity
package mpmc

import (
	"context"
	"fmt"
	"runtime"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sync/atomic"
	"time"
)

// Creates a new queue
func New[T any](namespace []string, initialCapacity uint64, minCapacity, maxCapacity int) (new *Queue[T], err error) {
	qInst, err := newQueueInst[T](namespace, initialCapacity)
	if err != nil {
		return
	}

	// Setup container where both pointers are to the same queue (initially)
	new = &Queue[T]{}
	new.ActiveRead.Store(qInst)
	new.ActiveWrite.Store(qInst)
	new.migrateCh.Store(make(chan struct{}, 1))
	new.minimumSize = minCapacity
	new.maximumSize = maxCapacity

	return
}

// Allocates new capacity queue (migration handled automatically asynchronously)
func (container *Queue[T]) mutateSize(newCapacity uint64) (err error) {
	// Safety, don't do anything if a migration is already in progress
	if container.ActiveRead.Load() != container.ActiveWrite.Load() {
		return
	}

	// Grab old namespace
	ns := container.ActiveWrite.Load().Namespace

	// Create the new size (empty) queue
	qInst, err := newQueueInst[T](ns, newCapacity)
	if err != nil {
		return
	}

	// Create a fresh migration channel
	container.migrateCh.Store(make(chan struct{}, 1))

	// Set old queue to draining (triggers producer to reload pointer)
	container.ActiveWrite.Load().draining.Store(true)

	// Assign ActiveWrite to new size queue instance
	// Migration is handled automatically by consumers
	container.ActiveWrite.Store(qInst)
	return
}

// Creates new queue instance (no container A/B - Write/Read)
func newQueueInst[T any](namespace []string, capacity uint64) (new *QueueInst[T], err error) {
	if (capacity & (capacity - 1)) != 0 {
		err = fmt.Errorf("capacity must be a power of two")
		return
	}
	if capacity < 2 {
		err = fmt.Errorf("capacity must be greater than or equal to 2")
		return
	}

	buf := make([]cell[T], capacity)
	for i := uint64(0); i < capacity; i++ {
		buf[i].seq.Store(i)
	}

	new = &QueueInst[T]{
		Namespace: append(namespace, global.NSQueue),
		Size:      int(capacity),
		mask:      atomic.Uint64{},
		buf:       buf,
		notEmpty:  make(chan struct{}, 1),
		Metrics:   &MetricStorage{},
	}
	new.mask.Store(capacity - 1)
	return
}

// Poll based wrapper around Push function to block until succeed (includes built-in poll interval)
func (container *Queue[T]) PushBlocking(ctx context.Context, value T, size int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if container.Push(value) { // try once
				container.ActiveWrite.Load().Metrics.Bytes.Add(uint64(size))
				return
			}
			time.Sleep(10 * time.Millisecond) // or backoff
		}
	}
}

// Attempts to write an element (non success = queue full)
func (container *Queue[T]) Push(value T) (success bool) {
	var queue *QueueInst[T]

	// Retry to get valid pointer
	for {
		queue = container.ActiveWrite.Load()
		if !queue.draining.Load() {
			break
		}
		// Loaded queue pointer is not valid to write to
		runtime.Gosched() // yield
	}

	queue.Metrics.PushAttempts.Add(1)

	var pos, seq uint64
	var cell *cell[T]

	for {
		pos = queue.tail.Load()
		cell = &queue.buf[pos&queue.mask.Load()]
		seq = cell.seq.Load()

		if seq == pos {
			if queue.tail.CompareAndSwap(pos, pos+1) {
				queue.Metrics.PushSuccess.Add(1)
				break
			}
			queue.Metrics.PushCASRetries.Add(1)
		} else if seq < pos {
			queue.Metrics.PushFull.Add(1)
			success = false // queue full
			return
		} else {
			queue.Metrics.PushSeqAhead.Add(1)
			runtime.Gosched() // yield then retry
		}
	}

	cell.data = value
	cell.seq.Store(pos + 1)
	queue.Metrics.Depth.Add(1)

	// notify blocked consumers, non-blocking
	select {
	case queue.notEmpty <- struct{}{}:
	default:
	}

	success = true
	return
}

// Attempts to read an element. Returns false if empty.
func (container *Queue[T]) Pop(ctx context.Context) (out T, success bool) {
	var pos, seq uint64
	var cell *cell[T]

	for {
		queue := container.ActiveRead.Load()
		queue.Metrics.PopAttempts.Add(1)

		pos = queue.head.Load()
		cell = &queue.buf[pos&queue.mask.Load()]
		seq = cell.seq.Load()
		readySeq := pos + 1

		if seq == readySeq {
			if queue.head.CompareAndSwap(pos, pos+1) {
				out = cell.data
				cell.seq.Store(pos + queue.mask.Load() + 1)

				queue.Metrics.PopSuccess.Add(1)
				ok := atomics.Subtract(&queue.Metrics.Depth, 1, 4) // max retries set at 4
				if !ok {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
						"failed to decrement queue depth metric after successful pop\n")
				}

				// Check for last pop on migrating queue (wake consumers)
				if queue.draining.Load() {
					if queue.head.Load() == queue.tail.Load() {
						migrateSignal := container.migrateCh.Load().(chan struct{})

						select {
						case migrateSignal <- struct{}{}:
						default: // channel full, skip (only need one signal)
						}
					}
				}

				success = true
				return
			}
			queue.Metrics.PopCASRetries.Add(1)
			continue
		}

		// queue empty: wait for signal or context cancel
		if seq < readySeq {
			queue.Metrics.PopEmpty.Add(1)
			migrateSignal := container.migrateCh.Load().(chan struct{})

			select {
			case <-ctx.Done():
				success = false
				return
			case <-queue.notEmpty:
				queue.Metrics.PopWaitSignals.Add(1)
				continue // retry after being signaled
			case <-migrateSignal:
				// Finish migration by flipping read to write pointer
				// Woken via channel from another/this consumer (the one that reads the last queue item)
				container.ActiveRead.Store(container.ActiveWrite.Load())
				continue
			}
		}

		// seq > readySeq, another consumer ahead, retry
		queue.Metrics.PopSeqBehind.Add(1)
	}
}
