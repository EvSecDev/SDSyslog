package listener

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	BusyNs         atomic.Uint64 // sum of ns spent doing anything
	ValidPackets   atomic.Uint64 // number of received packets that passed validation
	InvalidPackets atomic.Uint64 // number of received packets that failed validation
	SumNs          atomic.Uint64 // sum of elapsed ns for all ops
	MaxNs          atomic.Uint64 // max observed op duration
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	busyNs := instance.Metrics.BusyNs.Swap(0)
	valid := instance.Metrics.ValidPackets.Swap(0)
	invalid := instance.Metrics.InvalidPackets.Swap(0)
	sumNs := instance.Metrics.SumNs.Swap(0)
	maxNs := instance.Metrics.MaxNs.Swap(0)

	// Record read time
	recordTime := time.Now()

	// Percent worker was busy
	busyPct := (float64(busyNs) / float64(interval.Nanoseconds())) * 100

	total := valid + invalid
	var avgNs uint64
	if total > 0 {
		avgNs = sumNs / total
	}

	collection = []metrics.Metric{
		{
			Name:        "busy_time_percent",
			Description: "Total time spent doing anything in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      busyPct,
				Unit:     "%",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
		{
			Name:        "valid_packets_total",
			Description: "Total packets that passed basic validation in the interval",
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
			Name:        "invalid_packets_total",
			Description: "Total packets that failed basic validation in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      invalid,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        "total_packets",
			Description: "Total packets received in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      total,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        "elapsed_time_sum_ns",
			Description: "Total time spent validating packets in the interval",
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
			Description: "Average time spent validating packets in the interval",
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
			Description: "Maximum (seen) time spent validating packets in the interval",
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
