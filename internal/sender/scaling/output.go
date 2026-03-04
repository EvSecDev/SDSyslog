package scaling

import (
	"context"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/output"
	"time"
)

func scaleOutput(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, outMgr *output.Manager) {
	instanceList := outMgr.Instances.Load()
	instances := *instanceList

	// No scaling if we are at the min/max
	if len(instances) == int(outMgr.Config.MaxInstanceCount.Load()) ||
		len(instances) == int(outMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	metrics := metricStore.Search(mpmc.MTDepth, []string{logctx.NSRecv, logctx.NSmOutput}, time.Now().Add(-time.Duration(pastNIntervals)*interval), time.Now())
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
		addedID := outMgr.AddInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up output (added id %d)\n", addedID)
	} else if scaleDown {
		removedID := outMgr.RemoveLastInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down output (removed id %d)\n", removedID)
	}

	// Scale inbox queue as well
	outMgr.InQueue.ScaleCapacity(ctx)
}
