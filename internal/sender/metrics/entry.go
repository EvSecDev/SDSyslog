// Gathers instance metrics and saves to central registry
package metrics

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/assembler"
	"sdsyslog/internal/sender/ingest"
	"sdsyslog/internal/sender/output"
	"time"
)

func New(ingestMgr *ingest.Manager, asmMgr *assembler.Manager, outputMgr *output.Manager, interval time.Duration, maximumMetricAge time.Duration) (new *Gatherer) {
	new = &Gatherer{
		Registry:  metrics.New(),
		Ingest:    ingestMgr,
		Assembler: asmMgr,
		Output:    outputMgr,
		Interval:  interval,
		Retention: maximumMetricAge,
	}
	return
}

func (gatherer *Gatherer) Run(ctx context.Context) {
	ctx = logctx.AppendCtxTag(ctx, logctx.NSMetric)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	interval := gatherer.Interval

	// For metric data retention checks
	var tickCount int

	for {
		now := time.Now()

		// Current aligned slice
		currentSlice := now.Truncate(interval)

		// Next boundary
		nextSlice := currentSlice.Add(interval)

		// Sleep only until next boundary
		sleep := time.Until(nextSlice)
		if sleep < 0 {
			sleep = 0
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}

		// Recompute "now" after wake up
		now = time.Now()
		timeSlice := now.Truncate(interval)

		gatherer.Registry.NewTimeSlice(timeSlice, interval)
		gatherer.runIntervalTasks(ctx, timeSlice, interval)

		// Retention check periodically
		tickCount++
		if tickCount >= 30 {
			gatherer.Registry.Prune(now, gatherer.Retention)
			tickCount = 0
		}
	}
}

// Read and calculate metrics for each pipeline component
func (gatherer *Gatherer) runIntervalTasks(ctx context.Context, timeSlice time.Time, interval time.Duration) {
	// Record panics and continue on next interval
	defer func() {
		if fatalError := recover(); fatalError != nil {
			stack := debug.Stack()
			logctx.LogStdErr(ctx,
				"panic in sender metric collector thread: %v\n%s", fatalError, stack)
		}
	}()

	// Gatherer is started post-daemon pipeline startup, therefore certain pointers have to be initialized already (startup is run synchronously)

	// Ingest

	// File input
	gatherer.Ingest.FileSourceMu.RLock()
	for _, inst := range gatherer.Ingest.FileSources {
		if inst == nil {
			// Should only happen at daemon shutdown
			continue
		}
		m1 := inst.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m1)
	}
	gatherer.Ingest.FileSourceMu.RUnlock()

	// Journal input
	if gatherer.Ingest.JournalSource != nil {
		m0 := gatherer.Ingest.JournalSource.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m0)
	}

	// Raw Input
	if gatherer.Ingest.RawSource != nil {
		m0 := gatherer.Ingest.RawSource.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m0)
	}

	// Packaging
	m1 := gatherer.Assembler.InQueue.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, m1)

	assemInstances := gatherer.Assembler.Instances.Load()
	for _, instance := range *assemInstances {
		m2 := instance.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m2)
	}

	// Output
	collection := gatherer.Output.InQueue.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, collection)

	outputInstances := gatherer.Output.Instances.Load()
	for _, instance := range *outputInstances {
		m2 := instance.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m2)
	}
}
