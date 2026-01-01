package shared

import (
	"sdsyslog/internal/sender/managers/ingest"
	"sdsyslog/internal/sender/managers/out"
	"sdsyslog/internal/sender/managers/packaging"
)

// Pipeline component trackers (reverse order)
type Managers struct {
	Out   *out.InstanceManager
	Assem *packaging.InstanceManager
	In    *ingest.InstanceManager
}
