package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/listener"
	"strconv"
	"time"
)

func scaleListener(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, inMgr *listener.Manager) {
	instanceList := inMgr.Instances.Load()
	instances := *instanceList

	// No scaling if we are at the min/max
	if len(instances) == int(inMgr.Config.MaxInstanceCount.Load()) ||
		len(instances) == int(inMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]float64, 0, len(instances))

	for id := 0; id <= len(instances)-1; id++ {
		metrics := metricStore.Search(
			listener.MTBusyPct,
			[]string{logctx.NSRecv, logctx.NSmIngest, strconv.Itoa(id)},
			time.Now().Add(-time.Duration(pastNIntervals)*interval),
			time.Now(),
		)

		if len(metrics) < pastNIntervals {
			// Not enough data, skip this instance
			continue
		}

		// Keep only last x entries
		metrics = metrics[len(metrics)-pastNIntervals:]

		// Extract raw float64 values for this instance
		vals := make([]float64, pastNIntervals)
		for i, m := range metrics {
			vals[i] = m.Value.Raw.(float64)
		}

		instValues = append(instValues, vals)
	}

	values := make([]float64, pastNIntervals)

	for i := 0; i < pastNIntervals; i++ {
		column := make([]float64, 0, len(instValues))
		for _, inst := range instValues {
			column = append(column, inst[i])
		}

		values[i] = calc.TrimmedMeanFloat64(column, 0.10)
	}

	// Determine scaling direction
	scaleUp, scaleDown := listener.Trend(values)

	if scaleUp {
		addedID, err := inMgr.AddInstance()
		if err != nil {
			logctx.LogStdErr(ctx, "Failed to scale up listener instances: %w\n", err)
			return
		}
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up listener (added id %d)\n", addedID)
	} else if scaleDown {
		removedID := inMgr.RemoveLastInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down listener (removed id %d)\n", removedID)
	}
}
