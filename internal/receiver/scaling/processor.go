package scaling

import (
	"context"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/processor"
	"strings"
	"time"
)

func scaleProcessor(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, procMgr *processor.Manager) {
	instanceList := procMgr.Instances.Load()
	instances := *instanceList

	// No scaling if we are at the min/max
	if len(instances) == int(procMgr.Config.MaxInstanceCount.Load()) ||
		len(instances) == int(procMgr.Config.MinInstanceCount.Load()) {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	metrics := metricStore.Search(mpmc.MTDepth, []string{logctx.NSRecv, logctx.NSProc, logctx.NSQueue}, time.Now().Add(-time.Duration(pastNIntervals)*interval), time.Now())
	if len(metrics) < pastNIntervals {
		// Not enough data, ignoring
		return
	}

	// Extract values in order
	values := make([]uint64, 0, len(metrics))
	for _, m := range metrics {
		val, ok := m.Value.Raw.(uint64)
		if !ok {
			logctx.LogStdErr(ctx, "Failed to type assert metric %s (%s) to uint64: value=%+v type=%T\n",
				m.Name, strings.Join(m.Namespace, "/"), m.Value.Raw, m.Value.Raw)
			return
		}
		values = append(values, val)
	}

	// Determine scaling direction
	queue := procMgr.Inbox.ActiveWrite.Load()
	scaleUp, scaleDown := mpmc.Trend(values, queue.Size)

	if scaleUp {
		addedID := procMgr.AddInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled up processor (added id %d)\n", addedID)
	} else if scaleDown {
		removedID := procMgr.RemoveLastInstance()
		logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Scaled down processor (removed id %d)\n", removedID)
	}

	// Check queue for scaling
	procMgr.Inbox.ScaleCapacity(ctx)
}
