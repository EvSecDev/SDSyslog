package scaling

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/managers/out"
	"time"
)

func scaleOutput(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, outMgr *out.InstanceManager) {
	// No scaling if we are at the min/max
	outMgr.Mu.Lock()
	instanceCount := len(outMgr.Instances)
	outMgr.Mu.Unlock()
	if instanceCount == outMgr.MaxInstCount || instanceCount == outMgr.MinInstCount {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	metrics := metricStore.Search("depth", []string{global.NSRecv, global.NSmOutput}, time.Now().Add(-time.Duration(pastNIntervals)*interval), time.Now())
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
	queue := outMgr.InQueue.ActiveWrite.Load()
	scaleUp, scaleDown := mpmc.Trend(values, queue.Size)

	if scaleUp {
		outMgr.AddInstance()
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled up output\n")
	} else if scaleDown {
		for instanceId := range outMgr.Instances {
			outMgr.RemoveInstance(instanceId)
			break
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down output\n")
	}

	// Scale inbox queue as well
	outMgr.InQueue.ScaleCapacity(ctx)
}
