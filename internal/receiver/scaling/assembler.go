package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/managers/defrag"
	"sdsyslog/internal/receiver/shard"
	"time"
)

func scaleAssembler(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, defragMgr *defrag.Manager) {
	// Grab required info
	instanceCount := len(defragMgr.RoutingView.GetNonDrainingIDs())

	// No scaling if we are at the min/max
	if instanceCount == int(defragMgr.Config.MaxInstanceCount.Load()) ||
		instanceCount == int(defragMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]uint64, 0, instanceCount)

	for _, id := range defragMgr.RoutingView.GetNonDrainingIDs() {
		metrics := metricStore.Search(
			shard.MTTotalBuckets,
			[]string{logctx.NSRecv, logctx.NSmDefrag, id},
			time.Now().Add(-time.Duration(pastNIntervals)*interval),
			time.Now(),
		)

		if len(metrics) < pastNIntervals {
			// Not enough data, skip this instance
			continue
		}

		// Keep only last x entries
		metrics = metrics[len(metrics)-pastNIntervals:]

		// Extract raw uint64 values for this instance
		vals := make([]uint64, pastNIntervals)
		for i, m := range metrics {
			vals[i] = m.Value.Raw.(uint64)
		}

		instValues = append(instValues, vals)
	}

	values := make([]uint64, pastNIntervals)

	for i := 0; i < pastNIntervals; i++ {
		column := make([]uint64, 0, len(instValues))
		for _, inst := range instValues {
			column = append(column, inst[i])
		}

		values[i] = calc.TrimmedMeanUint64(column, 0.10)
	}

	// Determine scaling direction
	scaleUp, scaleDown := shard.Trend(values)

	if scaleUp {
		newID := defragMgr.AddInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up assembler (added id %s)\n", newID)
	} else if scaleDown {
		delID := defragMgr.RemoveOldestInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down assembler (removed id %s)\n", delID)
	}
}
