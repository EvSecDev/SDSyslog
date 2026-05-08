// Commons for the sender daemon to get around import cycle
package shared

import (
	"sdsyslog/internal/iomodules/internallogger"
	"sdsyslog/internal/sender/assembler"
	"sdsyslog/internal/sender/ingest"
	"sdsyslog/internal/sender/output"
)

// Pipeline component trackers (reverse order)
type Managers struct {
	Out         *output.Manager
	Assem       *assembler.Manager
	In          *ingest.Manager
	LogInjector *internallogger.SenderInjector
}
