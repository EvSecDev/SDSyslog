package output

import (
	"sdsyslog/internal/metrics"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	TotalPackets   atomic.Uint64
	SumPacketBytes atomic.Uint64
	MaxPacketBytes atomic.Uint64
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	totalPkts := instance.Metrics.TotalPackets.Swap(0)
	sumPktSizeB := instance.Metrics.SumPacketBytes.Swap(0)
	maxPktSizeB := instance.Metrics.MaxPacketBytes.Swap(0)

	// Record read time
	recordTime := time.Now()

	var avgPktSize uint64
	if totalPkts > 0 {
		avgPktSize = sumPktSizeB / totalPkts
	}

	collection = []metrics.Metric{
		{
			Name:        "total_sent_packets",
			Description: "Total packets sent in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      totalPkts,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "sum_packet_size",
			Description: "Total size of all packets sent in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      sumPktSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "maximum_packet_size",
			Description: "Maximum (seen) size across all packets sent in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      maxPktSizeB,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Gauge,
			Timestamp: recordTime,
		},
		{
			Name:        "average_packet_size",
			Description: "Average size across all packets sent in the interval",
			Namespace:   instance.Namespace,
			Value: metrics.MetricValue{
				Raw:      avgPktSize,
				Unit:     "bytes",
				Interval: interval,
			},
			Type:      metrics.Summary,
			Timestamp: recordTime,
		},
	}

	return
}
