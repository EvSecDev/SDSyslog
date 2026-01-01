// Gathers instance metrics and saves to central registry
package metrics

import (
	"context"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/shared"
	"time"
)

func New(mgrs shared.Managers, interval time.Duration, maximumMetricAge time.Duration) (new *Gatherer) {
	new = &Gatherer{
		Registry:  metrics.New(),
		Mgrs:      mgrs,
		Interval:  interval,
		Retention: maximumMetricAge,
	}
	return
}

func (gatherer *Gatherer) Run(ctx context.Context) {
	ctx = logctx.AppendCtxTag(ctx, global.NSMetric)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	// Track last run times for each interval
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
				"panic in receiver metric collector thread: %v\n%s", fatalError, stack)
		}
	}()

	// Gatherer is started post-daemon pipeline startup, therefore certain pointers have to be initialized already (startup is run synchronously)

	// Listener
	gatherer.Mgrs.Input.Mu.Lock() // Ensure instances don't disappear mid-read
	for _, instance := range gatherer.Mgrs.Input.Instances {
		if instance.Listener.Metrics == nil {
			continue
		}

		// Listener
		m1 := instance.Listener.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m1)
	}
	gatherer.Mgrs.Input.Mu.Unlock()

	// Processor
	// Queue
	m1 := gatherer.Mgrs.Proc.Inbox.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, m1)

	var procCollect []metrics.Metric // collection for all instances
	gatherer.Mgrs.Proc.Mu.Lock()
	for _, instance := range gatherer.Mgrs.Proc.Instances {
		if instance.Processor.Metrics == nil {
			continue
		}

		// Processor
		m2 := instance.Processor.CollectMetrics(interval)
		procCollect = append(procCollect, m2...)
	}
	gatherer.Mgrs.Proc.Mu.Unlock()
	gatherer.Registry.Add(timeSlice, procCollect)

	// Defrag
	gatherer.Mgrs.Defrag.Mu.Lock()
	var collection []metrics.Metric // collection for all pairs
	for _, instance := range gatherer.Mgrs.Defrag.InstancePairs {
		if instance.Shard.Metrics == nil || instance.Assembler.Metrics == nil {
			continue
		}

		// Shard
		m1 := instance.Shard.CollectMetrics(interval)
		collection = append(collection, m1...)

		// Assembler
		m2 := instance.Assembler.CollectMetrics(interval)
		collection = append(collection, m2...)
	}
	gatherer.Mgrs.Defrag.Mu.Unlock()

	// Save collected metrics to the registry
	gatherer.Registry.Add(timeSlice, collection)

	// Output
	// Inbox Queue
	metrics := gatherer.Mgrs.Output.Queue.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, metrics)

	// Worker
	if gatherer.Mgrs.Output.Instance.Worker != nil {
		if gatherer.Mgrs.Output.Instance.Worker.Metrics != nil {
			metrics := gatherer.Mgrs.Output.Instance.Worker.CollectMetrics(interval)
			gatherer.Registry.Add(timeSlice, metrics)
		}
	}
}
