package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/managers/in"
	"strconv"
	"time"
)

func scaleListener(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, inMgr *in.InstanceManager) {
	// No scaling if we are at the min/max
	inMgr.Mu.Lock()
	instanceCount := len(inMgr.Instances)
	inMgr.Mu.Unlock()
	if instanceCount == inMgr.MaxInstCount || instanceCount == inMgr.MinInstCount {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	instValues := make([][]float64, 0, len(inMgr.Instances))

	for id := range inMgr.Instances {
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
		// Picking effectively a random instance (map keys)
		for instanceId := range inMgr.Instances {
			inMgr.RemoveInstance(instanceId)
			break
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down listener\n")
	}
}
