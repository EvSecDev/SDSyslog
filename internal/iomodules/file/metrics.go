package file

import (
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	LinesRead atomic.Uint64 // number of lines read from source
	Success   atomic.Uint64 // number of messages processed successfully
}

const (
	MTLinesRead string = "lines_read"
	MTSuc       string = "success_processed"
)

func (mod *InModule) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	lines := mod.metrics.LinesRead.Swap(0)
	suc := mod.metrics.Success.Swap(0)

	// Record read time
	recordTime := time.Now()

	namespace := logctx.GetTagList(mod.ctx)

	collection = []metrics.Metric{
		{
			Name:        MTLinesRead,
			Description: "Total lines read from file in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      lines,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        MTSuc,
			Description: "Total processed messages extracted from file in the interval",
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
