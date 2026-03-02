// Watches metrics for each pipeline stage to decide whether to add more or less instances within configured bounds
package scaling

import (
	"context"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/shared"
	"time"
)

func New(metrics *metrics.Registry, interval time.Duration, managers shared.Managers) (new *Instance) {
	new = &Instance{
		MetricStore:  metrics,
		PollInterval: interval,
		Managers:     managers,
	}
	return
}

func (instance *Instance) Run(ctx context.Context) {
	ticker := time.NewTicker(instance.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Listeners
			scaleListener(ctx, instance.MetricStore, instance.PollInterval, instance.Managers.Input)

			// Processors
			scaleProcessor(ctx, instance.MetricStore, instance.PollInterval, instance.Managers.Proc)

			// Assemblers+Shards
			scaleAssembler(ctx, instance.MetricStore, instance.PollInterval, instance.Managers.Assembler)
			scaleTimeouts(ctx, instance.MetricStore, instance.PollInterval, instance.Managers.Assembler)

			// Output Queue
			instance.Managers.Output.Inbox.ScaleCapacity(ctx)
		}
	}
}
