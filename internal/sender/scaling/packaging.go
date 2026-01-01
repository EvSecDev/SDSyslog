package scaling

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/managers/packaging"
	"time"
)

func scaleAssembler(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, assemMgr *packaging.InstanceManager) {
	// No scaling if we are at the min/max
	assemMgr.Mu.Lock()
	instanceCount := len(assemMgr.Instances)
	assemMgr.Mu.Unlock()
	if instanceCount == assemMgr.MaxInstCount || instanceCount == assemMgr.MinInstCount {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	metrics := metricStore.Search("depth", []string{global.NSRecv, global.NSmPack}, time.Now().Add(-time.Duration(pastNIntervals)*interval), time.Now())
	if len(metrics) < pastNIntervals {
		// Not enough data, ignoring
		return
	}

	// Extract values in order
	values := make([]uint64, 0, len(metrics))
	for _, m := range metrics {
		values = append(values, m.Value.Raw.(uint64))
	}

	// Determine scaling direction
	queue := assemMgr.InQueue.ActiveWrite.Load()
	scaleUp, scaleDown := mpmc.Trend(values, queue.Size)

	if scaleUp {
		assemMgr.AddInstance()
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled up assembler\n")
	} else if scaleDown {
		// Picking effectively a random instance (map keys)
		for instanceId := range assemMgr.Instances {
			assemMgr.RemoveInstance(instanceId)
			break
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down assembler\n")
	}

	// Scale inbox queue as well
	assemMgr.InQueue.ScaleCapacity(ctx)
}
