package metrics

import (
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/assembler"
	"sdsyslog/internal/sender/ingest"
	"sdsyslog/internal/sender/output"
	"time"
)

type Gatherer struct {
	Interval  time.Duration     // Polling interval to gather metrics at
	Retention time.Duration     // Maximum time to maintain metrics for
	Registry  *metrics.Registry // Storage for metric data
	Ingest    *ingest.Manager
	Assembler *assembler.Manager
	Output    *output.Manager
}
