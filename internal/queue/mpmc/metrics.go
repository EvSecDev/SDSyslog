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
	PushFull       atomic.Uint64 // queue full (seq < pos)
	PushCASRetries atomic.Uint64 // CAS failed (seq==pos but CAS failed)
	PushSeqAhead   atomic.Uint64 // seq > pos (consumer ahead)

	PopAttempts    atomic.Uint64 // every Pop call
	PopSuccess     atomic.Uint64 // CAS success
	PopEmpty       atomic.Uint64 // empty condition encountered
	PopWaitSignals atomic.Uint64 // woken up by notEmpty
	PopCASRetries  atomic.Uint64 // CAS failed
	PopSeqBehind   atomic.Uint64 // seq > readySeq (producer ahead)
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
		Depth, Bytes                                uint64
		PushAttempts, PushSuccess, PushFull         uint64
		PushCASRetries, PushSeqAhead                uint64
		PopAttempts, PopSuccess, PopEmpty           uint64
		PopWaitSignals, PopCASRetries, PopSeqBehind uint64
	}{}

	for _, q := range queues {
		agg.Depth += q.Metrics.Depth.Load()
		agg.Bytes += q.Metrics.Bytes.Load()
		agg.PushAttempts += q.Metrics.PushAttempts.Swap(0)
		agg.PushSuccess += q.Metrics.PushSuccess.Swap(0)
		agg.PushFull += q.Metrics.PushFull.Swap(0)
		agg.PushCASRetries += q.Metrics.PushCASRetries.Swap(0)
		agg.PushSeqAhead += q.Metrics.PushSeqAhead.Swap(0)
		agg.PopAttempts += q.Metrics.PopAttempts.Swap(0)
		agg.PopSuccess += q.Metrics.PopSuccess.Swap(0)
		agg.PopEmpty += q.Metrics.PopEmpty.Swap(0)
		agg.PopWaitSignals += q.Metrics.PopWaitSignals.Swap(0)
		agg.PopCASRetries += q.Metrics.PopCASRetries.Swap(0)
		agg.PopSeqBehind += q.Metrics.PopSeqBehind.Swap(0)
	}

	recordTime := time.Now()
	sec := interval.Seconds()

	pa := float64(max(agg.PushAttempts, 1))
	po := float64(max(agg.PopAttempts, 1))
	f := func(v uint64) float64 { return float64(v) }

	// Derived metrics
	pushThroughput := f(agg.PushSuccess) / sec
	popThroughput := f(agg.PopSuccess) / sec

	pushFullRatio := f(agg.PushFull) / pa
	popEmptyRatio := f(agg.PopEmpty) / po

	pushCASFailRatio := f(agg.PushCASRetries) / pa
	popCASFailRatio := f(agg.PopCASRetries) / po

	producerAheadRatio := f(agg.PushSeqAhead) / pa
	consumerAheadRatio := f(agg.PopSeqBehind) / po

	pushEfficiency := f(agg.PushSuccess) / pa
	popEfficiency := f(agg.PopSuccess) / po

	popWaitSuccessRatio := f(agg.PopWaitSignals) / float64(max(agg.PopEmpty, 1))

	burstiness := (f(agg.PushSeqAhead) + f(agg.PopSeqBehind)) / (pa + po)

	queueHealth := 1.0 -
		pushFullRatio*0.4 -
		popEmptyRatio*0.4 -
		(pushCASFailRatio+popCASFailRatio)*0.2

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

	// Raw
	add("depth", agg.Depth, "count", metrics.Gauge, "Current number of items in the queue")
	add("byte_sum", agg.Bytes, "bytes", metrics.Gauge, "Byte sum of all items in the queue")
	add("push_attempts", agg.PushAttempts, "count", metrics.Counter, "Total push attempts in the interval")
	add("push_success", agg.PushSuccess, "count", metrics.Counter, "Total push attempts that succeeded in the interval")
	add("push_full", agg.PushFull, "count", metrics.Counter, "Total push attempts that failed because the queue was full in the interval")
	add("push_cas_retries", agg.PushCASRetries, "count", metrics.Counter, "Sum of retries to push in the interval")
	add("push_seq_ahead", agg.PushSeqAhead, "count", metrics.Counter, "Total push retries due to sequence ahead in the interval")
	add("pop_attempts", agg.PopAttempts, "count", metrics.Counter, "Total pop attempts in the interval")
	add("pop_success", agg.PopSuccess, "count", metrics.Counter, "Total pop attempts that succeeded in the interval")
	add("pop_empty", agg.PopEmpty, "count", metrics.Counter, "Total pop attempts that failed because the queue was empty in the interval")
	add("pop_wait_signals", agg.PopWaitSignals, "count", metrics.Counter, "Sum of all times consumer was woken by producer in the interval")
	add("pop_cas_retries", agg.PopCASRetries, "count", metrics.Counter, "Sum of retries to pop in the interval")
	add("pop_seq_behind", agg.PopSeqBehind, "count", metrics.Counter, "Total pop retries due to consumer ahead in the interval")

	// Derived
	add("push_throughput", pushThroughput, "ops/sec", metrics.Gauge, "Average push operations per second in the interval")
	add("pop_throughput", popThroughput, "ops/sec", metrics.Gauge, "Average pop operations per second in the interval")
	add("push_full_ratio", pushFullRatio, "ratio", metrics.Gauge, "Portion of push attempts that failed due to full queue")
	add("pop_empty_ratio", popEmptyRatio, "ratio", metrics.Gauge, "Portion of pop attempts that failed due to empty queue")
	add("push_cas_fail_ratio", pushCASFailRatio, "ratio", metrics.Gauge, "Portion of failed CAS operations during push")
	add("pop_cas_fail_ratio", popCASFailRatio, "ratio", metrics.Gauge, "Portion of failed CAS operations during pop")
	add("producer_ahead_ratio", producerAheadRatio, "ratio", metrics.Gauge, "Portion of push retries due to producer being ahead")
	add("consumer_ahead_ratio", consumerAheadRatio, "ratio", metrics.Gauge, "Portion of pop retries due to consumer being ahead")
	add("push_efficiency", pushEfficiency, "ratio", metrics.Gauge, "Ratio of successful push attempts to total")
	add("pop_efficiency", popEfficiency, "ratio", metrics.Gauge, "Ratio of successful pop attempts to total")
	add("pop_wait_success_ratio", popWaitSuccessRatio, "ratio", metrics.Gauge, "Success of consumers being woken by producer")
	add("burstiness", burstiness, "ratio", metrics.Gauge, "Spikiness or irregularity in push/pop flow")
	add("queue_health", queueHealth, "ratio", metrics.Gauge, "Overall queue health based on CAS failures and full/empty events")

	return
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
