package shared

import (
	"sdsyslog/internal/receiver/managers/defrag"
	"sdsyslog/internal/receiver/managers/in"
	"sdsyslog/internal/receiver/managers/out"
	"sdsyslog/internal/receiver/managers/proc"
	"sdsyslog/internal/receiver/shard/fiprrecv"
)

// Pipeline component trackers (reverse order)
type Managers struct {
	Output *out.Manager
	Defrag *defrag.Manager
	Proc   *proc.Manager
	Input  *in.Manager
	FIPR   *fiprrecv.Instance
}
