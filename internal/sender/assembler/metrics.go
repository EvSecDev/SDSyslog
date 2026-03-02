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

// Metric Names
const (
	MTTotalMsgs   string = "total_messages"
	MTTotalFrags  string = "sum_fragments"
	MTMaxFragsMsg string = "maximum_fragment_count"
	MTSumMsgSize  string = "sum_message_size"
	MTMaxMsgSize  string = "maximum_message_size"
	MTSumWorkTime string = "elapsed_time_sum_ns"
	MTMaxWorkTime string = "elapsed_time_max_ns"
)

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	totalMsgs := instance.Metrics.TotalMessages.Swap(0)
	sumMsgSizeB := instance.Metrics.TotalMsgSizeBytes.Swap(0)
	maxMsgSizeB := instance.Metrics.MaxMsgSizeBytes.Swap(0)
	sumFragCtn := instance.Metrics.TotalFragmentCtn.Swap(0)
	maxFragCtn := instance.Metrics.MaxFragmentCtn.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTTotalMsgs,
			Description: "Total received messages in the interval",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      totalMsgs,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTSumMsgSize,
			Description: "Total of all message sizes in the interval",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      sumMsgSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTMaxMsgSize,
			Description: "Maximum (seen) of all message sizes in the interval",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      maxMsgSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTTotalFrags,
			Description: "Total fragments of all messages in the interval",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      sumFragCtn,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTMaxFragsMsg,
			Description: "Maximum (seen) fragment count of all messages in the interval",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      maxFragCtn,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
	}
	return
}
