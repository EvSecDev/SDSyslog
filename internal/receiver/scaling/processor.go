package scaling

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/managers/proc"
	"time"
)

func scaleProcessor(ctx context.Context, metricStore *metrics.Registry, interval time.Duration, procMgr *proc.InstanceManager) {
	// No scaling if we are at the min/max
	procMgr.Mu.Lock()
	instanceCount := len(procMgr.Instances)
	procMgr.Mu.Unlock()
	if instanceCount == procMgr.MaxInstCount || instanceCount == procMgr.MinInstCount {
		return
	}

	const pastNIntervals = 5

	// Get the last x scaling polling intervals worth of load data and average
	metrics := metricStore.Search("depth", []string{global.NSRecv, global.NSProc, global.NSQueue}, time.Now().Add(-time.Duration(pastNIntervals)*interval), time.Now())
	if len(metrics) < pastNIntervals {
		// Not enough data, ignoring
		return
	}

	// Extract values in order
	values := make([]uint64, 0, len(metrics))
	for _, m := range metrics {
		values = append(values, m.Value.Raw.(uint64))
	}

	// Determine scaling direction
	queue := procMgr.Inbox.ActiveWrite.Load()
	scaleUp, scaleDown := mpmc.Trend(values, queue.Size)

	if scaleUp {
		procMgr.AddInstance()
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled up processor\n")
	} else if scaleDown {
		// Picking effectively a random instance (map keys)
		for instanceId := range procMgr.Instances {
			procMgr.RemoveInstance(instanceId)
			break
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Scaled down processor\n")
	}

	// Check queue for scaling
	procMgr.Inbox.ScaleCapacity(ctx)
}
