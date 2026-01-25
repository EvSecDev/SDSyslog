package lifecycle

import "time"

const (
	DefaultMaxWaitForUpdate time.Duration = 10 * time.Second // Max allowed child startup time
	ReadyMessage            string        = "READY"
	EnvNameReadinessFD      string        = "READY_FD"
	EnvNameSelfUpdate       string        = "UPDATING_CHILD-PID"
)
