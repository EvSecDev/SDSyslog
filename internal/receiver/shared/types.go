package shared

import (
	"sdsyslog/internal/receiver/managers/defrag"
	"sdsyslog/internal/receiver/managers/in"
	"sdsyslog/internal/receiver/managers/out"
	"sdsyslog/internal/receiver/managers/proc"
)

// Pipeline component trackers (reverse order)
type Managers struct {
	Output *out.InstanceManager
	Defrag *defrag.InstanceManager
	Proc   *proc.InstanceManager
	Input  *in.InstanceManager
}
