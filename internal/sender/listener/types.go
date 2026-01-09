package listener

import (
	"io"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
)

type FileInstance struct {
	Namespace       []string
	Source          io.ReadSeekCloser
	SourceFilePath  string
	SourceStateFile string
	Outbox          *mpmc.Queue[global.ParsedMessage]
	Metrics         *MetricStorage
}

type JrnlInstance struct {
	Namespace []string
	Journal   io.ReadCloser
	StateFile string
	Outbox    *mpmc.Queue[global.ParsedMessage]
	Metrics   *MetricStorage
}
