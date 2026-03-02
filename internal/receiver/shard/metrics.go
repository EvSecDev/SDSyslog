package shard

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	Bytes                  atomic.Uint64 // Current byte size of the queue
	TotalBuckets           atomic.Uint64 // Current number of buckets in the queue
	WaitingBuckets         atomic.Uint64 // Current number of filled buckets waiting to be processed
	TimedOutBuckets        atomic.Uint64 // Total buckets that were timed out instead of all fragments being received
	SumFragmentTimeSpacing atomic.Uint64 // Sum of time between message fragments
	PushCount              atomic.Uint64 // Total items pushed (or attempted to push) to the queue
	PopCount               atomic.Uint64 // Total items popped (or attempted to pop) from the queue
}

// Metric Names
const (
	MTBytes            string = "total_bytes"
	MTTotalBuckets     string = "total_buckets"
	MTWaitingBuckets   string = "waiting_buckets"
	MTTimedOutBuckets  string = "timed_out_buckets"
	MTTimeBtwFragments string = "sum_time_between_fragments"
	MTPushCnt          string = "push_ctn"
	MTPopCnt           string = "pop_ctn"
)

func (queue *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	totalBytes := queue.Metrics.Bytes.Load()
	totalBuckets := queue.Metrics.TotalBuckets.Load()
	waitingBuckets := queue.Metrics.WaitingBuckets.Load()
	timedOutBuckets := queue.Metrics.TimedOutBuckets.Swap(0)
	sumFragmentSpacing := queue.Metrics.SumFragmentTimeSpacing.Swap(0)
	popCtn := queue.Metrics.PopCount.Swap(0)
	pushCtn := queue.Metrics.PushCount.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTBytes,
			Description: "Total bytes currently in the queue (includes internal structure overheads)",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalBytes,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTTotalBuckets,
			Description: "Total buckets currently in the queue (not counting ones waiting for processing)",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalBuckets,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTWaitingBuckets,
			Description: "Total buckets waiting to be processed",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      waitingBuckets,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTTimedOutBuckets,
			Description: "Total buckets that did not receive all fragments of a message in the interval",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      timedOutBuckets,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTTimeBtwFragments,
			Description: "Sum of time to arrival between fragments in the interval",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      sumFragmentSpacing,
				Unit:     "ns",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTPushCnt,
			Description: "Total buckets sent into the queue in the interval",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      pushCtn,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTPopCnt,
			Description: "Total buckets received from the queue in the interval",
			Namespace:   queue.Namespace,
			Value: metrics.MetricValue{
				Raw:      popCtn,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
	}
	return
}
