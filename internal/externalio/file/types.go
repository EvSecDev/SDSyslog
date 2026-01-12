package file

import (
	"io"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
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
	outbox    *mpmc.Queue[global.ParsedMessage]
	metrics   MetricStorage
}
