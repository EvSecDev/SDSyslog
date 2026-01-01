package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/managers/defrag"
	"sdsyslog/internal/receiver/shard"
	"strconv"
	"time"
)

func scaleAssembler(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, defragMgr *defrag.InstanceManager) {
	// Grab required info under lock
	defragMgr.Mu.Lock()
	instanceCount := len(defragMgr.InstancePairs)
	var instanceIDs []int
	for id := range defragMgr.InstancePairs {
		instanceIDs = append(instanceIDs, id)
	}
	defragMgr.Mu.Unlock()

	// No scaling if we are at the min/max
	if instanceCount == defragMgr.MaxInstCount || instanceCount == defragMgr.MinInstCount {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]uint64, 0, instanceCount)

	for id := range instanceIDs {
		metrics := metricStore.Search(
			"total_buckets",
			[]string{global.NSRecv, global.NSmDefrag, strconv.Itoa(id)},
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
		defragMgr.AddInstance()
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled up assembler\n")
	} else if scaleDown {
		// Picking just the first (valid) one
		defragMgr.Mu.Lock()
		for instanceId := range defragMgr.InstancePairs {
			defragMgr.RemoveInstance(instanceId)
			break
		}
		defragMgr.Mu.Unlock()
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down assembler\n")
	}
}
