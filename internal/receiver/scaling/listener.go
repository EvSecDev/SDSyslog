package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/listener"
	"strconv"
	"time"
)

func scaleListener(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, inMgr *listener.Manager) {
	inMgr.Mu.RLock()
	instances := inMgr.Instances
	instanceCount := len(instances)
	inMgr.Mu.RUnlock()

	// No scaling if we are at the min/max
	if instanceCount == int(inMgr.Config.MaxInstanceCount.Load()) ||
		instanceCount == int(inMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]float64, 0, len(instances))

	for id := 0; id <= instanceCount-1; id++ {
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
		_, err := inMgr.AddInstance()
		if err != nil {
			logctx.LogStdErr(ctx, "Failed to scale up listener instances: %w\n", err)
			return
		}
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up listener\n")
	} else if scaleDown {
		instanceId, err := random.NumberInRange(0, instanceCount-1)
		if err != nil {
			logctx.LogStdErr(ctx, "Failed to generate random instance ID in instance map: %w\n", err)
			return
		}
		inMgr.RemoveInstance(instanceId)

		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down listener\n")
	}
}
