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

// Changes packet deadline value based on how often buckets are being timed out
func scaleTimeouts(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, defragMgr *defrag.InstanceManager) {
	// No scaling if we are at the min/max
	currentDeadline := defragMgr.PacketDeadline.Load()
	if currentDeadline <= int64(global.DefaultMinPacketDeadline) || currentDeadline >= int64(global.DefaultMaxPacketDeadline) {
		return
	}

	// Search params
	const pastNIntervals = 5
	start := time.Now().Add(-time.Duration(pastNIntervals) * interval)
	end := time.Now()

	// Aggregated across all instances
	aggSumSpacing := make([][]uint64, pastNIntervals)
	aggFragments := make([][]uint64, pastNIntervals)
	aggTimeouts := make([][]uint64, pastNIntervals)

	for id := range defragMgr.InstancePairs {
		ns := []string{global.NSRecv, global.NSmDefrag, strconv.Itoa(id)}

		sumSpacingMetrics := metricStore.Search("sum_time_between_fragments", ns, start, end)
		fragmentsMetrics := metricStore.Search("push_ctn", ns, start, end)
		timeoutsMetrics := metricStore.Search("timed_out_buckets", ns, start, end)

		if len(sumSpacingMetrics) < pastNIntervals ||
			len(fragmentsMetrics) < pastNIntervals ||
			len(timeoutsMetrics) < pastNIntervals {
			continue
		}

		// Keep only last N intervals
		sumSpacingMetrics = sumSpacingMetrics[len(sumSpacingMetrics)-pastNIntervals:]
		fragmentsMetrics = fragmentsMetrics[len(fragmentsMetrics)-pastNIntervals:]
		timeoutsMetrics = timeoutsMetrics[len(timeoutsMetrics)-pastNIntervals:]

		// Aggregate per interval
		for i := 0; i < pastNIntervals; i++ {
			if v, ok := sumSpacingMetrics[i].Value.Raw.(uint64); ok {
				aggSumSpacing[i] = append(aggSumSpacing[i], v)
			}
			if v, ok := fragmentsMetrics[i].Value.Raw.(uint64); ok {
				aggFragments[i] = append(aggFragments[i], v)
			}
			if v, ok := timeoutsMetrics[i].Value.Raw.(uint64); ok {
				aggTimeouts[i] = append(aggTimeouts[i], v)
			}
		}
	}

	// Compute trimmed mean for each interval
	finalSumSpacing := make([]uint64, pastNIntervals)
	finalFragments := make([]uint64, pastNIntervals)
	finalTimeouts := make([]uint64, pastNIntervals)

	for i := 0; i < pastNIntervals; i++ {
		if len(aggSumSpacing[i]) > 0 {
			finalSumSpacing[i] = calc.TrimmedMeanUint64(aggSumSpacing[i], 0.10)
		}
		if len(aggFragments[i]) > 0 {
			finalFragments[i] = calc.TrimmedMeanUint64(aggFragments[i], 0.10)
		}
		if len(aggTimeouts[i]) > 0 {
			finalTimeouts[i] = calc.TrimmedMeanUint64(aggTimeouts[i], 0.10)
		}
	}

	stepUp, stepDown := shard.TrendLatency(finalSumSpacing, finalFragments, finalTimeouts)

	deadlineDur := time.Duration(currentDeadline)
	if stepUp {
		newDeadline := deadlineDur + 10*time.Millisecond
		defragMgr.PacketDeadline.Store(int64(newDeadline))
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled up packet deadline time from %dms to %dms\n", deadlineDur.Milliseconds(), newDeadline)
	} else if stepDown {
		newDeadline := deadlineDur - 10*time.Millisecond
		defragMgr.PacketDeadline.Store(int64(newDeadline))
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down packet deadline time from %dms to %dms\n", deadlineDur.Milliseconds(), newDeadline)
	}
}
