package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/listener"
	"strconv"
	"strings"
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
		metricResults := metricStore.Search(
			listener.MTBusyPct,
			[]string{logctx.NSRecv, logctx.NSmIngest, strconv.Itoa(id)},
			time.Now().Add(-time.Duration(pastNIntervals)*interval),
			time.Now(),
		)

		if len(metricResults) < pastNIntervals {
			// Not enough data, skip this instance
			continue
		}

		// Keep only last x entries
		metricResults = metricResults[len(metricResults)-pastNIntervals:]

		// Extract raw float64 values for this instance
		vals := make([]float64, pastNIntervals)
		for index, metric := range metricResults {
			var ok bool
			vals[index], ok = metric.Value.Raw.(float64)
			if !ok {
				logctx.LogStdErr(ctx, "Failed to type assert metric %s (%s) to float64: value=%+v type=%T\n",
					metric.Name, strings.Join(metric.Namespace, "/"), metric.Value.Raw, metric.Value.Raw)
				return
			}
		}

		instValues = append(instValues, vals)
	}

	values := make([]float64, pastNIntervals)

	for intervalIndex := range pastNIntervals {
		column := make([]float64, 0, len(instValues))
		for _, inst := range instValues {
			column = append(column, inst[intervalIndex])
		}

		values[intervalIndex] = calc.TrimmedMeanFloat64(column, 0.10)
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
