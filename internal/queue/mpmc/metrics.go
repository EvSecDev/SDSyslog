package mpmc

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	Capacity atomic.Uint64 // Current size of active queue
	Depth    atomic.Uint64 // Current items in queue
	Bytes    atomic.Uint64 // Current byte size in queue (just data)

	PushAttempts      atomic.Uint64 // every Push call
	PushSuccess       atomic.Uint64 // CAS success
	PushCASRetries    atomic.Uint64 // CAS failed (seq==pos but CAS failed)
	PushSeqBehindTail atomic.Uint64 // not yet available
	PushStaleRetries  atomic.Uint64 // seq>pos

	PopAttempts       atomic.Uint64 // every Pop call
	PopSuccess        atomic.Uint64 // CAS success
	PopCASRetries     atomic.Uint64 // CAS failed
	PopEmptySeqBehind atomic.Uint64 // slot reuse / capacity boundary
	PopStaleRetries   atomic.Uint64 // seq>pos
}

// Metric Names
const (
	MTSize  string = "capacity"
	MTDepth string = "depth"
	MTBytes string = "total_bytes"

	MTPushAttempt       string = "push_attempts"
	MTPushSuc           string = "push_success"
	MTPushCASRetry      string = "push_cas_retries"
	MTPushSeqBehindTail string = "push_seq_behind_tail"
	MTPushStaleRetries  string = "push_stale_retries"

	MTPopAttempt      string = "pop_attempts"
	MTPopSuc          string = "pop_success"
	MTPopCASRetry     string = "pop_cas_retries"
	MTPopEmptySeq     string = "pop_empty_seq_behind"
	MTPopStaleRetries string = "pop_stale_retries"
)

func (container *Queue[T]) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	queues := []*QueueInst[T]{container.ActiveWrite.Load()}
	readQueue := container.ActiveRead.Load()
	// If different, include read queue for aggregation
	if readQueue != queues[0] {
		queues = append(queues, readQueue)
	}

	// Only for active
	currentCapacity := queues[0].Metrics.Capacity.Load()

	// Aggregate metrics across all queues
	agg := struct {
		Depth, Bytes                                                                   uint64
		PushAttempts, PushSuccess, PushCASRetries, PushSeqBehindTail, PushStaleRetries uint64
		PopAttempts, PopSuccess, PopCASRetries, PopEmptySeqBehind, PopStaleRetries     uint64
	}{}

	for _, q := range queues {
		agg.Depth += q.Metrics.Depth.Load()
		agg.Bytes += q.Metrics.Bytes.Load()

		agg.PushAttempts += q.Metrics.PushAttempts.Swap(0)
		agg.PushSuccess += q.Metrics.PushSuccess.Swap(0)
		agg.PushCASRetries += q.Metrics.PushCASRetries.Swap(0)
		agg.PushSeqBehindTail += q.Metrics.PushSeqBehindTail.Swap(0)
		agg.PushStaleRetries += q.Metrics.PushStaleRetries.Swap(0)

		agg.PopAttempts += q.Metrics.PopAttempts.Swap(0)
		agg.PopSuccess += q.Metrics.PopSuccess.Swap(0)
		agg.PopCASRetries += q.Metrics.PopCASRetries.Swap(0)
		agg.PopEmptySeqBehind += q.Metrics.PopEmptySeqBehind.Swap(0)
		agg.PopStaleRetries += q.Metrics.PopStaleRetries.Swap(0)
	}

	recordTime := time.Now()

	// Helper to add metrics
	add := func(name string, raw interface{}, unit string, t metrics.MetricType, description string) {
		collection = append(collection, metrics.Metric{
			Name:        name,
			Description: description,
			Namespace:   queues[0].Namespace,
			Type:        t,
			Timestamp:   recordTime,
			Value: metrics.MetricValue{
				Raw:      raw,
				Unit:     unit,
				Interval: interval,
			},
		})
	}

	add(MTSize, currentCapacity, "capacity", metrics.Summary, "Current active queue max capacity (total allocated entries) at time of metric collection")
	add(MTDepth, agg.Depth, "count", metrics.Gauge, "Current number of items in the queue")
	add(MTBytes, agg.Bytes, "bytes", metrics.Gauge, "Byte sum of all items in the queue")

	add(MTPushAttempt, agg.PushAttempts, "count", metrics.Counter, "Total push attempts in the interval")
	add(MTPushSuc, agg.PushSuccess, "count", metrics.Counter, "Total push attempts that succeeded in the interval")
	add(MTPushCASRetry, agg.PushCASRetries, "count", metrics.Counter, "Sum of retries to push in the interval")

	add(MTPushSeqBehindTail, agg.PushSeqBehindTail, "count", metrics.Counter, "Push observed a sequence value behind the expected tail position, indicating the slot is not yet available for writing (writer ahead)")
	add(MTPushStaleRetries, agg.PushStaleRetries, "count", metrics.Counter, "Push observed sequence ahead of local position (seq > pos), indicating another writer advanced the tail; retry due to stale local view of ring state.")

	add(MTPopAttempt, agg.PopAttempts, "count", metrics.Counter, "Total pop attempts in the interval")
	add(MTPopSuc, agg.PopSuccess, "count", metrics.Counter, "Total pop attempts that succeeded in the interval")
	add(MTPopCASRetry, agg.PopCASRetries, "count", metrics.Counter, "Sum of retries to pop in the interval")

	add(MTPopEmptySeq, agg.PopEmptySeqBehind, "count", metrics.Counter, "Pop observed sequence behind expected read position (seq < readySeq), indicating the slot is not yet visible/ready for consumption (read-before-write or visibility lag).")
	add(MTPopStaleRetries, agg.PopStaleRetries, "count", metrics.Counter, "Pop observed sequence ahead of expected head position (seq > readySeq), indicating another consumer advanced the head; retry due to stale local view of ring state.")

	return
}
