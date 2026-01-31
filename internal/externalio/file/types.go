package file

import (
	"io"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type OutModule struct {
	sink        io.WriteCloser
	batchBuffer *[]string
}

type InModule struct {
	Namespace []string
	sink      io.ReadSeekCloser
	filePath  string
	stateFile string
	outbox    *mpmc.Queue[protocol.Message]
	metrics   MetricStorage
}
