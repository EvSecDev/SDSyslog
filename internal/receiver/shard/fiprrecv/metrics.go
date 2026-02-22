package fiprrecv

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	Connections       atomic.Uint64 // Number of remote inbound connections
	AcceptedFragments atomic.Uint64 // Number of remote inbound fragments accepted
	RejectedFragments atomic.Uint64 // Number of remote inbound fragments rejected
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	totalConns := instance.Metrics.Connections.Swap(0)
	totalAccepted := instance.Metrics.AcceptedFragments.Swap(0)
	totalRejected := instance.Metrics.RejectedFragments.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        "total_connections",
			Description: "Total connections from any remote shard",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalConns,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "accepted_fragments",
			Description: "Total number of accepted remote fragments in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalAccepted,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "rejected_fragments",
			Description: "Total number of rejected remote fragments in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalRejected,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		},
	}
	return
}
