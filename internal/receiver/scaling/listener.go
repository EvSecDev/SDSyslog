package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/managers/in"
	"strconv"
	"time"
)

func scaleListener(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, inMgr *in.InstanceManager) {
	inMgr.Mu.Lock()
	instances := inMgr.Instances
	maxInstances := inMgr.MaxInstCount
	minInstances := inMgr.MinInstCount
	instanceCount := len(instances)
	inMgr.Mu.Unlock()

	// No scaling if we are at the min/max
	if instanceCount == maxInstances || instanceCount == minInstances {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]float64, 0, len(instances))

	for id := 0; id <= instanceCount-1; id++ {
		metrics := metricStore.Search(
			"busy_time_percent",
			[]string{global.NSRecv, global.NSmIngest, strconv.Itoa(id)},
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
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Failed to scale up listener instances: %w\n", err)
			return
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled up listener\n")
	} else if scaleDown {
		instanceId, err := random.NumberInRange(0, instanceCount-1)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Failed to generate random instance ID in instance map: %w\n", err)
			return
		}
		inMgr.RemoveInstance(instanceId)

		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down listener\n")
	}
}
