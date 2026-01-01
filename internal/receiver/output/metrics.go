package output

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ReceivedMessages     atomic.Uint64
	SuccessfulFileWrites atomic.Uint64
	SuccessfulJrnlWrites atomic.Uint64
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	recvMsgs := instance.Metrics.ReceivedMessages.Swap(0)
	fileWrites := instance.Metrics.SuccessfulFileWrites.Swap(0)
	jrnlWrites := instance.Metrics.SuccessfulJrnlWrites.Swap(0)

	totalWrites := fileWrites + jrnlWrites

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        "received_messages",
			Description: "Total messages received",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      recvMsgs,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        "written_messages",
			Description: "Total writes to any outputs (across all outputs)",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        "success_file_writes",
			Description: "Total writes to any file outputs",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      fileWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        "success_journal_writes",
			Description: "Total writes to journal output",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      jrnlWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
	}
	return
}
