package mpmc

import "sync/atomic"

type cell[T any] struct {
	seq  atomic.Uint64
	data T
}

type QueueInst[T any] struct {
	Namespace []string
	Size      int
	mask      atomic.Uint64
	buf       []cell[T]
	head      atomic.Uint64
	tail      atomic.Uint64
	notEmpty  chan struct{}
	draining  atomic.Bool // Gates producers from writing to this queue
	Metrics   *MetricStorage
}

// Container for split read/write views.
// ActiveWrite is the pointer to the queue that is currently accepting writes.
// ActiveRead is the pointer to the queue that is currently accepting reads.
// Pointers can either be pointed at the same queue or different queues (for scaling up/down).
type Queue[T any] struct {
	ActiveWrite atomic.Pointer[QueueInst[T]] // Pointer to queue for producers
	ActiveRead  atomic.Pointer[QueueInst[T]] // Pointer to queue for consumers
	migrateCh   atomic.Value                 // Buffered channel, used to wake a consumer once migration is ready to complete (flip read pointer to write one)
	minimumSize int                          // Lower configurable bound for scaling
	maximumSize int                          // Upper configurable bound for scaling
}
