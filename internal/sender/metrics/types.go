package metrics

import (
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/managers/ingest"
	"sdsyslog/internal/sender/managers/out"
	"sdsyslog/internal/sender/managers/packaging"
	"time"
)

type Gatherer struct {
	Interval  time.Duration     // Polling interval to gather metrics at
	Retention time.Duration     // Maximum time to maintain metrics for
	Registry  *metrics.Registry // Storage for metric data
	Ingest    *ingest.InstanceManager
	Packaging *packaging.InstanceManager
	Output    *out.InstanceManager
}
