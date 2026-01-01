package listener

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	LinesRead atomic.Uint64 // number of lines read from source
	Success   atomic.Uint64 // number of messages processed successfully
}

func (instance *FileInstance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	lines := instance.Metrics.LinesRead.Swap(0)
	suc := instance.Metrics.Success.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        "lines_read",
			Description: "Total lines read from sources in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      lines,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
		{
			Name:        "success_processed",
			Description: "Total processed messages extracted from sources in the interval",
			Namespace:   instance.Namespace,
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
