package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/shard"
	"strings"
	"time"
)

func scaleAssembler(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, asmMgr *assembler.Manager) {
	// Grab required info
	instanceCount := len(asmMgr.RoutingView.GetNonDrainingIDs())

	// No scaling if we are at the min/max
	if instanceCount == int(asmMgr.Config.MaxInstanceCount.Load()) ||
		instanceCount == int(asmMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]uint64, 0, instanceCount)

	for _, id := range asmMgr.RoutingView.GetNonDrainingIDs() {
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
			var ok bool
			vals[i], ok = m.Value.Raw.(uint64)
			if !ok {
				logctx.LogStdErr(ctx, "Failed to type assert metric %s (%s) to uint64: value=%+v type=%T\n",
					m.Name, strings.Join(m.Namespace, "/"), m.Value.Raw, m.Value.Raw)
				return
			}
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
		newID := asmMgr.AddInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up assembler (added id %s)\n", newID)
	} else if scaleDown {
		delID := asmMgr.RemoveOldestInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down assembler (removed id %s)\n", delID)
	}
}
