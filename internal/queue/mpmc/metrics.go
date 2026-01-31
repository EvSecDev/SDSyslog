package mpmc

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	Depth atomic.Uint64 // Current items in queue
	Bytes atomic.Uint64 // Current byte size in queue (just data)

	PushAttempts   atomic.Uint64 // every Push call
	PushSuccess    atomic.Uint64 // CAS success
	PushCASRetries atomic.Uint64 // CAS failed (seq==pos but CAS failed)

	PopAttempts   atomic.Uint64 // every Pop call
	PopSuccess    atomic.Uint64 // CAS success
	PopCASRetries atomic.Uint64 // CAS failed
}

func (container *Queue[T]) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	queues := []*QueueInst[T]{container.ActiveWrite.Load()}
	readQueue := container.ActiveRead.Load()
	// If different, include read queue for aggregation
	if readQueue != queues[0] {
		queues = append(queues, readQueue)
	}

	// Aggregate metrics across all queues
	agg := struct {
		Depth, Bytes                              uint64
		PushAttempts, PushSuccess, PushCASRetries uint64
		PopAttempts, PopSuccess, PopCASRetries    uint64
	}{}

	for _, q := range queues {
		agg.Depth += q.Metrics.Depth.Load()
		agg.Bytes += q.Metrics.Bytes.Load()
		agg.PushAttempts += q.Metrics.PushAttempts.Swap(0)
		agg.PushSuccess += q.Metrics.PushSuccess.Swap(0)
		agg.PushCASRetries += q.Metrics.PushCASRetries.Swap(0)
		agg.PopAttempts += q.Metrics.PopAttempts.Swap(0)
		agg.PopSuccess += q.Metrics.PopSuccess.Swap(0)
		agg.PopCASRetries += q.Metrics.PopCASRetries.Swap(0)
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

	add("depth", agg.Depth, "count", metrics.Gauge, "Current number of items in the queue")
	add("byte_sum", agg.Bytes, "bytes", metrics.Gauge, "Byte sum of all items in the queue")
	add("push_attempts", agg.PushAttempts, "count", metrics.Counter, "Total push attempts in the interval")
	add("push_success", agg.PushSuccess, "count", metrics.Counter, "Total push attempts that succeeded in the interval")
	add("push_cas_retries", agg.PushCASRetries, "count", metrics.Counter, "Sum of retries to push in the interval")
	add("pop_attempts", agg.PopAttempts, "count", metrics.Counter, "Total pop attempts in the interval")
	add("pop_success", agg.PopSuccess, "count", metrics.Counter, "Total pop attempts that succeeded in the interval")
	add("pop_cas_retries", agg.PopCASRetries, "count", metrics.Counter, "Sum of retries to pop in the interval")

	return
}
