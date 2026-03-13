package lifecycle

import (
	"syscall"
	"time"
)

const (
	DefaultSignalChannelSize int           = 10
	DefaultMaxWaitForUpdate  time.Duration = 10 * time.Second // Max allowed child startup time
	ReadyMessage             string        = "READY"
	EnvNameReadinessFD       string        = "READY_FD"
	EnvNameSelfUpdate        string        = "UPDATING_CHILD_PID"

	FullUpdateSignal   syscall.Signal = syscall.SIGHUP
	PinKeyReloadSignal syscall.Signal = syscall.SIGUSR1
)
