// Gathers instance metrics and saves to central registry
package metrics

import (
	"context"
	"runtime/debug"
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
				"panic in receiver metric collector thread: %v\n%s", fatalError, stack)
		}
	}()

	// Gatherer is started post-daemon pipeline startup, therefore certain pointers have to be initialized already (startup is run synchronously)

	// Listener
	inputInstances := gatherer.Mgrs.Input.Instances.Load()
	for _, instance := range *inputInstances {
		m1 := instance.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m1)
	}
	m2 := gatherer.Mgrs.Input.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, m2)

	// Processor
	// Queue
	m1 := gatherer.Mgrs.Proc.Inbox.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, m1)

	procInstances := gatherer.Mgrs.Proc.Instances.Load()
	for _, instance := range *procInstances {
		m2 := instance.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m2)
	}
	m3 := gatherer.Mgrs.Proc.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, m3)

	// Defrag
	var collection []metrics.Metric // collection for all pairs
	for _, instancePair := range gatherer.Mgrs.Assembler.RoutingView.GetInstancePairs() {
		if instancePair == nil {
			continue
		}

		// Shard
		m1 := instancePair.Shard.CollectMetrics(interval)
		collection = append(collection, m1...)

		// Assembler
		m2 := instancePair.CollectMetrics(interval)
		collection = append(collection, m2...)
	}
	m3 = gatherer.Mgrs.Assembler.CollectMetrics(interval)
	collection = append(collection, m3...)

	// Save collected metrics to the registry
	gatherer.Registry.Add(timeSlice, collection)

	// FIPR (if present)
	if gatherer.Mgrs.FIPR != nil {
		m1 := gatherer.Mgrs.FIPR.CollectMetrics(interval)
		gatherer.Registry.Add(timeSlice, m1)
	}

	// Output
	// Inbox Queue
	metrics := gatherer.Mgrs.Output.Inbox.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, metrics)

	// Instance
	metrics = gatherer.Mgrs.Output.Instance.CollectMetrics(interval)
	gatherer.Registry.Add(timeSlice, metrics)
}
