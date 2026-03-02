package output

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ReceivedMessages      atomic.Uint64
	SuccessfulFileWrites  atomic.Uint64
	SuccessfulJrnlWrites  atomic.Uint64
	SuccessfulBeatsWrites atomic.Uint64
}

const (
	MTRecvMsgs       string = "received_messages"
	MTSentMsgs       string = "written_messages"
	MTFileWritesSuc  string = "success_file_writes"
	MTJrnlWritesSuc  string = "success_journal_writes"
	MTBeatsWritesSuc string = "success_beats_writes"
)

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	recvMsgs := instance.Metrics.ReceivedMessages.Swap(0)
	fileWrites := instance.Metrics.SuccessfulFileWrites.Swap(0)
	jrnlWrites := instance.Metrics.SuccessfulJrnlWrites.Swap(0)
	beatsWrites := instance.Metrics.SuccessfulBeatsWrites.Swap(0)

	totalWrites := fileWrites + jrnlWrites

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTRecvMsgs,
			Description: "Total messages received",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      recvMsgs,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTSentMsgs,
			Description: "Total writes to any outputs (across all outputs)",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      totalWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTFileWritesSuc,
			Description: "Total writes to any file outputs",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      fileWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTJrnlWritesSuc,
			Description: "Total writes to journal output",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      jrnlWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTBeatsWritesSuc,
			Description: "Total writes to beats output",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      beatsWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
	}
	return
}
