// Watches metrics for each pipeline stage to decide whether to add more or less instances within configured bounds
package scaling

import (
	"context"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/shared"
	"time"
)

func New(metrics *metrics.Registry, interval time.Duration, managers shared.Managers, logicalCpuCtn int) (new *Instance) {
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
			// Assembler
			scaleAssembler(ctx, instance.MetricStore, instance.PollInterval, instance.Managers.Assem)

			// Output (TODO)
			// DEBUG

			// Output Queue
			instance.Managers.Out.InQueue.ScaleCapacity(ctx)
		}
	}
}
