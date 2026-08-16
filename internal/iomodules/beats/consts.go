package beats

import "time"

const (
	DefaultAddress     string        = "localhost:5044"
	minStartupDuration time.Duration = 5 * time.Second
	startupRetryDelay  time.Duration = 2 * time.Second
	dialTimeout        time.Duration = 3 * time.Second
)
