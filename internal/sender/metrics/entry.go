// Gathers instance metrics and saves to central registry
package metrics

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/managers/ingest"
	"sdsyslog/internal/sender/managers/out"
	"sdsyslog/internal/sender/managers/packaging"
	"time"
)

func New(ingestMgr *ingest.InstanceManager, packMgr *packaging.InstanceManager, outputMgr *out.InstanceManager, interval time.Duration, maximumMetricAge time.Duration) (new *Gatherer) {
	new = &Gatherer{
		Registry:  metrics.New(),
		Ingest:    ingestMgr,
		Packaging: packMgr,
		Output:    outputMgr,
		Interval:  interval,
		Retention: maximumMetricAge,
	}
	return
}

func (gatherer *Gatherer) Run(ctx context.Context) {
	ctx = logctx.AppendCtxTag(ctx, global.NSMetric)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	// Tracking last interval run time
	lastRun := time.Now()

	ticker := time.NewTicker(gatherer.Interval / 2) // Use polling interval half of desired record interval
	defer ticker.Stop()

	// Counter to track how many ticks have passed (for retention)
	var tickCount int

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if now.Sub(lastRun) >= gatherer.Interval {
				timeSlice := gatherer.Registry.NewTimeSlice(now, gatherer.Interval)

				lastRun = now
				go gatherer.runIntervalTasks(ctx, timeSlice, gatherer.Interval)
			}

			// Conduct old metric evaluations and cleanup
			tickCount++
			if tickCount >= 30 {
				gatherer.Registry.Prune(now, gatherer.Retention)
				tickCount = 0 // Reset the counter after cleanup
			}
		}
	}
}

// Read and calculate metrics for each pipeline component
func (gatherer *Gatherer) runIntervalTasks(ctx context.Context, timeSlice time.Time, interval time.Duration) {
	// Record panics and continue on next interval
	defer func() {
		if fatalError := recover(); fatalError != nil {
			stack := debug.Stack()
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"panic in sender metric collector thread: %v\n%s", fatalError, stack)
		}
	}()

	// Gatherer is started post-daemon pipeline startup, therefore certain pointers have to be initialized already (startup is run synchronously)

	// Ingest

	// File input
	gatherer.Ingest.Mu.Lock()
	for _, inst := range gatherer.Ingest.FileSources {
		if inst == nil {
			// Should only happen at daemon shutdown
			continue
		}
		m1 := inst.Worker.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m1)
	}
	gatherer.Ingest.Mu.Unlock()

	// Journal input
	if gatherer.Ingest.JournalSource != nil {
		if gatherer.Ingest.JournalSource.Worker != nil {
			m0 := gatherer.Ingest.JournalSource.Worker.CollectMetrics(interval)
			gatherer.Registry.Add(timeSlice, m0)
		}
	}

	// Packaging
	m1 := gatherer.Packaging.InQueue.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, m1)

	gatherer.Packaging.Mu.Lock()
	for _, instance := range gatherer.Packaging.Instances {
		m2 := instance.Worker.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m2)
	}
	gatherer.Packaging.Mu.Unlock()

	// Output
	collection := gatherer.Output.InQueue.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, collection)

	gatherer.Output.Mu.Lock()
	for _, instance := range gatherer.Output.Instances {
		m2 := instance.Worker.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m2)
	}
	gatherer.Output.Mu.Unlock()
}
