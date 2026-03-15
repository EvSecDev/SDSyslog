package assembler

import (
	"sdsyslog/internal/logctx"
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
	MTInstanceCount  string = "instance_count"
	MTPacketDeadline string = "packet_deadline"
)

func (manager *Manager) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Record read time
	recordTime := time.Now()

	namespace := logctx.GetTagList(manager.ctx)

	collection = []metrics.Metric{
		{
			Name:        MTInstanceCount,
			Description: "Number of running instances at the time of metric collection",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      len(manager.RoutingView.GetAllIDs()),
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTPacketDeadline,
			Description: "Packet deadline (maximum time between fragments) at the time of metric collection",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      time.Duration(manager.Config.PacketDeadline.Load()).Nanoseconds(),
				Unit:     "ns",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
	}
	return
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	valid := instance.Metrics.ProcessedBuckets.Swap(0)
	sumNs := instance.Metrics.SumNs.Swap(0)
	maxNs := instance.Metrics.MaxNs.Swap(0)

	namespace := logctx.GetTagList(instance.ctx)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTProcessBuckets,
			Description: "Number of buckets successfully processed in the interval",
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
			Name:        MTSumWorkTime,
			Description: "Total time spent processing buckets in the interval (excludes pop/push from queues)",
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
			Description: "Maximum (seen) time spent processing buckets in the interval (excludes pop/push from queues)",
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
