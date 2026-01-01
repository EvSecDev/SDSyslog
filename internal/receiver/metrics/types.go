package metrics

import (
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/receiver/shared"
	"time"
)

type Gatherer struct {
	Interval  time.Duration     // Polling interval to gather metrics at
	Retention time.Duration     // Maximum time to maintain metrics for
	Registry  *metrics.Registry // Storage for metric data
	Mgrs      shared.Managers   // Has pointers to all the managers
}
