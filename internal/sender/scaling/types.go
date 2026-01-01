package scaling

import (
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/sender/shared"
	"time"
)

type Instance struct {
	PollInterval time.Duration
	MetricStore  *metrics.Registry
	Managers     shared.Managers
}
