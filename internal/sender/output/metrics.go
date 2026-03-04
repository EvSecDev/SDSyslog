package output

import (
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	TotalPackets   atomic.Uint64
	SumPacketBytes atomic.Uint64
	MaxPacketBytes atomic.Uint64
}

// Metric Names
const (
	MTSentPackets string = "total_sent_packets"
	MTSumPktSizes string = "sum_packet_size"
	MTMaxPktSize  string = "maximum_packet_size"
)

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	if instance == nil {
		return
	}

	namespace := logctx.GetTagList(instance.ctx)

	// Read and clear
	totalPkts := instance.Metrics.TotalPackets.Swap(0)
	sumPktSizeB := instance.Metrics.SumPacketBytes.Swap(0)
	maxPktSizeB := instance.Metrics.MaxPacketBytes.Swap(0)

	// Record read time
	recordTime := time.Now()

	collection = []metrics.Metric{
		{
			Name:        MTSentPackets,
			Description: "Total packets sent in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      totalPkts,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTSumPktSizes,
			Description: "Total size of all packets sent in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      sumPktSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        MTMaxPktSize,
			Description: "Maximum (seen) size across all packets sent in the interval",
			Namespace:   namespace,
			Value: metrics.MetricValue{
				Raw:      maxPktSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
	}

	return
}
