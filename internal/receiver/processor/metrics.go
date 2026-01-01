package processor

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ValidPayloads atomic.Uint64 // number of received payloads that passed validation
	SumNs         atomic.Uint64 // sum of elapsed ns for all ops
	MaxNs         atomic.Uint64 // max observed op duration
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	valid := instance.Metrics.ValidPayloads.Swap(0)
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
			Name:        "valid_payloads_total",
			Description: "Total validated (parsed) packets in the interval",
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
			Description: "Total time spent processing packets in the interval",
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
			Description: "Average time spent processing packets in the interval",
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
			Description: "Maximum (seen) time spent processing packets in the interval",
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
