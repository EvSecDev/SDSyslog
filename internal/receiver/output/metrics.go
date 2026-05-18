package output

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ReceivedMessages       atomic.Uint64
	SuccessfulFileWrites   atomic.Uint64
	SuccessfulJrnlWrites   atomic.Uint64
	SuccessfulBeatsWrites  atomic.Uint64
	SuccessfulRawWrites    atomic.Uint64
	SuccessfulNotifyWrites atomic.Uint64
	Dropped                atomic.Uint64
}

const (
	MTRecvMsgs        string = "received_messages"
	MTWrittenMsgs     string = "written_messages"
	MTFileWritesSuc   string = "success_file_writes"
	MTJrnlWritesSuc   string = "success_journal_writes"
	MTBeatsWritesSuc  string = "success_beats_writes"
	MTRawWritesSuc    string = "success_raw_writes"
	MTNotifyWritesSuc string = "success_notify_writes"
)

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	recvMsgs := instance.Metrics.ReceivedMessages.Swap(0)
	fileWrites := instance.Metrics.SuccessfulFileWrites.Swap(0)
	jrnlWrites := instance.Metrics.SuccessfulJrnlWrites.Swap(0)
	beatsWrites := instance.Metrics.SuccessfulBeatsWrites.Swap(0)
	rawWrites := instance.Metrics.SuccessfulRawWrites.Swap(0)
	notifyWrites := instance.Metrics.SuccessfulNotifyWrites.Swap(0)
	dropped := instance.Metrics.Dropped.Swap(0)

	totalWrites := fileWrites + jrnlWrites + beatsWrites + rawWrites + notifyWrites

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTRecvMsgs,
			Description: "Total messages received from assemblers",
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
			Name:        MTWrittenMsgs,
			Description: "Sum of messages successfully written to all outputs",
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
		{
			Name:        MTRawWritesSuc,
			Description: "Total writes to raw output",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      rawWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTNotifyWritesSuc,
			Description: "Total writes to dbus notify output",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      notifyWrites,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        metrics.MTDropped,
			Description: metrics.DescDropped,
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      dropped,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
	}
	return
}
