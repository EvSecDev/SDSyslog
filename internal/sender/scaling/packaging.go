package scaling

import (
	"context"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/assembler"
	"time"
)

func scaleAssembler(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, assemMgr *assembler.Manager) {
	instanceList := assemMgr.Instances.Load()
	instances := *instanceList

	// No scaling if we are at the min/max
	if len(instances) == int(assemMgr.Config.MaxInstanceCount.Load()) ||
		len(instances) == int(assemMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	metrics := metricStore.Search(mpmc.MTDepth, []string{logctx.NSRecv, logctx.NSmPack}, time.Now().Add(-time.Duration(pastNIntervals)*interval), time.Now())
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
		addedID := assemMgr.AddInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up assembler (added id %d)\n", addedID)
	} else if scaleDown {
		removedID := assemMgr.RemoveLastInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down assembler (removed id %d)\n", removedID)
	}

	// Scale inbox queue as well
	assemMgr.InQueue.ScaleCapacity(ctx)
}
