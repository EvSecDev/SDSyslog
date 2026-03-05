package processor

import (
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ValidPayloads   atomic.Uint64 // number of received payloads that passed validation
	InvalidPayloads atomic.Uint64 // number of received payloads that failed validation
	SumNs           atomic.Uint64 // sum of elapsed ns for all ops
	MaxNs           atomic.Uint64 // max observed op duration
}

// Metric Names
const (
	MTValidPayloads   string = "valid_payloads_total"
	MTInvalidPayloads string = "invalid_payloads_total"
	MTSumWorkTime     string = "elapsed_time_sum_ns"
	MTMaxWorkTime     string = "elapsed_time_max_ns"
	MTInstanceCount   string = "instance_count"
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
	valid := instance.Metrics.ValidPayloads.Swap(0)
	invalid := instance.Metrics.InvalidPayloads.Swap(0)
	sumNs := instance.Metrics.SumNs.Swap(0)
	maxNs := instance.Metrics.MaxNs.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTValidPayloads,
			Description: "Total validated (parsed) packets in the interval",
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
			Name:        MTInvalidPayloads,
			Description: "Total invalid packets in the interval",
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
			Description: "Total time spent processing packets in the interval",
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
			Description: "Maximum (seen) time spent processing packets in the interval",
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
