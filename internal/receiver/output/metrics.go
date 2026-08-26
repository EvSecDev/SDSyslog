package output

import (
	"sdsyslog/internal/metrics"
	"sync"
	"sync/atomic"
	"time"
)

type MetricStorage struct {
	ReceivedMessages atomic.Uint64
	writeMutex       sync.Mutex
	successfulWrites map[string]*atomic.Uint64
	Dropped          atomic.Uint64
}

const (
	MTRecvMsgs     string = "received_messages"
	MTWrittenMsgs  string = "written_messages"
	MTRawWritesSuc string = "success_raw_writes" // Hard coded for testing only
)

func (instance *Instance) initializeMetrics() {
	instance.Metrics.writeMutex.Lock()
	defer instance.Metrics.writeMutex.Unlock()

	instance.Metrics.successfulWrites = make(map[string]*atomic.Uint64)
	for moduleName := range instance.outModules {
		instance.Metrics.successfulWrites[moduleName] = &atomic.Uint64{}
	}
}

func (instance *Instance) writeModuleMetrics(moduleName string, msgsWritten int) {
	instance.Metrics.writeMutex.Lock()
	defer instance.Metrics.writeMutex.Unlock()

	atomicVal, ok := instance.Metrics.successfulWrites[moduleName]
	if !ok {
		return
	}
	atomicVal.Add(uint64(msgsWritten))
	instance.Metrics.successfulWrites[moduleName] = atomicVal
}

func (instance *Instance) CollectMetrics(interval time.Duration) (collection []metrics.Metric) {
	// Read and clear
	recvMsgs := instance.Metrics.ReceivedMessages.Swap(0)
	dropped := instance.Metrics.Dropped.Swap(0)

	// Record read time
	recordTime := time.Now()

	instance.Metrics.writeMutex.Lock()
	collection = make([]metrics.Metric, 0, 3+len(instance.Metrics.successfulWrites))
	instance.Metrics.writeMutex.Unlock()
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

	instance.Metrics.writeMutex.Lock()
	defer instance.Metrics.writeMutex.Unlock()

	var totalWrites uint64
	for moduleName, moduleWritten := range instance.Metrics.successfulWrites {
		numWritten := moduleWritten.Swap(0)

		collection = append(collection, metrics.Metric{
			Name:        "success_" + moduleName + "_writes",
			Description: "Total writes to any " + moduleName + " outputs",
			Namespace:   instance.namespace,
			Value: metrics.MetricValue{
				Raw:      numWritten,
				Unit:     "count",
				Interval: interval,
			},
			Type:      metrics.Counter,
			Timestamp: recordTime,
		})
		totalWrites += numWritten
	}

	collection = append(collection, metrics.Metric{
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
	})
	return
}
