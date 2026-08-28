package scaling

import (
	"context"
	"sdsyslog/internal/calc"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/shard"
	"time"
)

// Changes packet deadline value based on how often buckets are being timed out
func scaleTimeouts(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, asmMgr *assembler.Manager) {
	// No scaling if we are at the min/max
	currentDeadline := asmMgr.Config.PacketDeadline.Load()
	if currentDeadline <= int64(global.DefaultMinPacketDeadline) ||
		currentDeadline >= int64(global.DefaultMaxPacketDeadline) {
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

	for _, id := range asmMgr.RoutingView.GetNonDrainingIDs() {
		ns := []string{logctx.NSRecv, logctx.NSmDefrag, id}

		sumSpacingMetrics := metricStore.Search(shard.MTTimeBtwFragments, ns, start, end)
		fragmentsMetrics := metricStore.Search(shard.MTPushCnt, ns, start, end)
		timeoutsMetrics := metricStore.Search(shard.MTTimedOutBuckets, ns, start, end)

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
		for intervalIndex := range pastNIntervals {
			if v, ok := sumSpacingMetrics[intervalIndex].Value.Raw.(uint64); ok {
				aggSumSpacing[intervalIndex] = append(aggSumSpacing[intervalIndex], v)
			}
			if v, ok := fragmentsMetrics[intervalIndex].Value.Raw.(uint64); ok {
				aggFragments[intervalIndex] = append(aggFragments[intervalIndex], v)
			}
			if v, ok := timeoutsMetrics[intervalIndex].Value.Raw.(uint64); ok {
				aggTimeouts[intervalIndex] = append(aggTimeouts[intervalIndex], v)
			}
		}
	}

	// Compute trimmed mean for each interval
	finalSumSpacing := make([]uint64, pastNIntervals)
	finalFragments := make([]uint64, pastNIntervals)
	finalTimeouts := make([]uint64, pastNIntervals)

	for intervalIndex := range pastNIntervals {
		if len(aggSumSpacing[intervalIndex]) > 0 {
			finalSumSpacing[intervalIndex] = calc.TrimmedMeanUint64(aggSumSpacing[intervalIndex], 0.10)
		}
		if len(aggFragments[intervalIndex]) > 0 {
			finalFragments[intervalIndex] = calc.TrimmedMeanUint64(aggFragments[intervalIndex], 0.10)
		}
		if len(aggTimeouts[intervalIndex]) > 0 {
			finalTimeouts[intervalIndex] = calc.TrimmedMeanUint64(aggTimeouts[intervalIndex], 0.10)
		}
	}

	stepUp, stepDown := shard.TrendLatency(finalSumSpacing, finalFragments, finalTimeouts)

	deadlineDur := time.Duration(currentDeadline)
	if stepUp {
		newDeadline := deadlineDur + 10*time.Millisecond
		asmMgr.Config.PacketDeadline.Store(int64(newDeadline))
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Scaled up packet deadline time from %dms to %dms\n", deadlineDur.Milliseconds(), newDeadline)
	} else if stepDown {
		newDeadline := deadlineDur - 10*time.Millisecond
		asmMgr.Config.PacketDeadline.Store(int64(newDeadline))
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Scaled down packet deadline time from %dms to %dms\n", deadlineDur.Milliseconds(), newDeadline)
	}
}
