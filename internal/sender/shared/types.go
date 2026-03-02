package shared

import (
	"sdsyslog/internal/sender/managers/ingest"
	"sdsyslog/internal/sender/managers/out"
	"sdsyslog/internal/sender/managers/packaging"
)

// Pipeline component trackers (reverse order)
type Managers struct {
	Out   *out.Manager
	Assem *packaging.Manager
	In    *ingest.Manager
}
