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

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	valid := instance.Metrics.ProcessedBuckets.Swap(0)
	sumNs := instance.Metrics.SumNs.Swap(0)
	maxNs := instance.Metrics.MaxNs.Swap(0)

	// Record read time
	recordTime := time.Now()

	var avgNs uint64
	if valid > 0 {
		avgNs = sumNs / valid
	}

	collection = []metrics.Metric{
		{
			Name:        "processed_buckets",
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
			Name:        "elapsed_time_sum_ns",
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
			Name:        "elapsed_time_avg_ns",
			Description: "Average time spent processing buckets in the interval (excludes pop/push from queues)",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      avgNs,
				Unit:     "ns",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
		{
			Name:        "elapsed_time_max_ns",
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
