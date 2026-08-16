package sender

import "time"

const (
	ShutdownTimeout          time.Duration = 5 * time.Second
	journalStateSaveInterval time.Duration = 1 * time.Hour

	DefaultOutputThrottlingThreshold int           = 25                    // Number of fragments for a message
	DefaultOutputThrottlingTime      time.Duration = 50 * time.Microsecond // Sleep between each fragment (packet)
)
