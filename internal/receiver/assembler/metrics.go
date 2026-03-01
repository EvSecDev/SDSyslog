package assembler

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ProcessedBuckets atomic.Uint64 // number of processed buckets
	SumNs            atomic.Uint64 // sum of elapsed ns for all ops
	MaxNs            atomic.Uint64 // max observed op duration
}

// Metric Names
const (
	MTProcessBuckets string = "processed_buckets"
	MTSumWorkTime    string = "elapsed_time_sum_ns"
	MTMaxWorkTime    string = "elapsed_time_max_ns"
)

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	valid := instance.Metrics.ProcessedBuckets.Swap(0)
	sumNs := instance.Metrics.SumNs.Swap(0)
	maxNs := instance.Metrics.MaxNs.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTProcessBuckets,
			Description: "Number of buckets successfully processed in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      valid,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTSumWorkTime,
			Description: "Total time spent processing buckets in the interval (excludes pop/push from queues)",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      sumNs,
				Unit:     "ns",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTMaxWorkTime,
			Description: "Maximum (seen) time spent processing buckets in the interval (excludes pop/push from queues)",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      maxNs,
				Unit:     "ns",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
	}
	return
}
