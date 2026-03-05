package listener

import (
	"sdsyslog/internal/logctx"
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

// Metric Names
const (
	MTBusyPct       string = "busy_time_percent"
	MTValidPkts     string = "valid_packets_total"
	MTInvalidPkts   string = "invalid_packets_total"
	MTSumWorkTime   string = "elapsed_time_sum_ns"
	MTMaxWorkTime   string = "elapsed_time_max_ns"
	MTInstanceCount string = "instance_count"
)

func (manager *Manager) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Record read time
	recordTime := time.Now()

	listPtr := manager.Instances.Load()
	var instCount int
	if listPtr != nil {
		instCount = len(*listPtr)
	}

	namespace := logctx.GetTagList(manager.ctx)

	collection = []metrics.Metric{
		{
			Name:        MTInstanceCount,
			Description: "Number of running instances at the time of metric collection",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      instCount,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
	}
	return
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	if instance == nil {
		return
	}

	namespace := logctx.GetTagList(instance.ctx)

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

	collection = []metrics.Metric{
		{
			Name:        MTBusyPct,
			Description: "Total time spent doing anything in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      busyPct,
				Unit:     "%",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
		{
			Name:        MTValidPkts,
			Description: "Total packets that passed basic validation in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      valid,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTInvalidPkts,
			Description: "Total packets that failed basic validation in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      invalid,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTSumWorkTime,
			Description: "Total time spent validating packets in the interval",
			Namespace:   namespace,
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
			Description: "Maximum (seen) time spent validating packets in the interval",
			Namespace:   namespace,
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
