package assembler

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	TotalMessages     atomic.Uint64 // Total of all complete messages
	TotalMsgSizeBytes atomic.Uint64 // Total of all message text size
	MaxMsgSizeBytes   atomic.Uint64 // Maximum seen message text size
	TotalFragmentCtn  atomic.Uint64 // Total of all fragments produced
	MaxFragmentCtn    atomic.Uint64 // Maximum seen fragment count for a given message
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	totalMsgs := instance.Metrics.TotalMessages.Swap(0)
	sumMsgSizeB := instance.Metrics.TotalMsgSizeBytes.Swap(0)
	maxMsgSizeB := instance.Metrics.MaxMsgSizeBytes.Swap(0)
	sumFragCtn := instance.Metrics.TotalFragmentCtn.Swap(0)
	maxFragCtn := instance.Metrics.MaxFragmentCtn.Swap(0)

	// Record read time
	recordTime := time.Now()

	var avgMsgSize uint64
	if totalMsgs > 0 {
		avgMsgSize = sumMsgSizeB / totalMsgs
	}

	collection = []metrics.Metric{
		{
			Name:        "total_messages",
			Description: "Total received messages in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalMsgs,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "sum_message_size",
			Description: "Total of all message sizes in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      sumMsgSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "maximum_message_size",
			Description: "Maximum (seen) of all message sizes in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      maxMsgSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "sum_fragments",
			Description: "Total fragments of all messages in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      sumFragCtn,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "maximum_fragment_count",
			Description: "Maximum (seen) fragment count of all messages in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      maxFragCtn,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "average_message_size",
			Description: "Average size of all messages in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      avgMsgSize,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
	}
	return
}
