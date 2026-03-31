package generic

import (
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	CompleteReads atomic.Uint64 // number of complete reads from source (delimited by EOF)
	Success       atomic.Uint64 // number of messages processed successfully
}

const (
	MTBatchesRead string = "complete_inputs"
	MTSuc         string = "success_processed"
)

func (mod *InModule) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	read := mod.metrics.CompleteReads.Swap(0)
	suc := mod.metrics.Success.Swap(0)

	// Record read time
	recordTime := time.Now()

	namespace := logctx.GetTagList(mod.ctx)

	collection = []metrics.Metric{
		{
			Name:        MTBatchesRead,
			Description: "Total read chunks from raw source in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      read,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTSuc,
			Description: "Total processed messages extracted from raw source in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      suc,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
	}
	return
}
