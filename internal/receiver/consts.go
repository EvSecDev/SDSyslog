package receiver

import (
	"sdsyslog/internal/global"
	"time"
)

const (
	DefaultReplayWindow         time.Duration = 10 * time.Minute
	DefaultPastValidityWindow   time.Duration = 12 * time.Hour
	DefaultFutureValidityWindow time.Duration = 4 * time.Hour
	DefaultSocketDir            string        = global.DefaultStateDir + "/ipc"
	ShutdownTimeout             time.Duration = 20 * time.Second

	DefaultOutputFailureDuration time.Duration = 10 * time.Minute // Span of time where consecutive and total output failures are considered fatal and program exits

	PinedKeysReqSeparator string = "|"
)
