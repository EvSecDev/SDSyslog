package shared

import (
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/output"
	"sdsyslog/internal/receiver/processor"
	"sdsyslog/internal/receiver/shard/fiprrecv"
)

// Pipeline component trackers (reverse order)
type Managers struct {
	Output    *output.Manager
	Assembler *assembler.Manager
	Proc      *processor.Manager
	Input     *listener.Manager
	FIPR      *fiprrecv.Instance
}
