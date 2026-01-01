package listener

import (
	"io"
	"sdsyslog/internal/queue/mpmc"
	"time"
)

type FileInstance struct {
	Namespace       []string
	Source          io.ReadSeekCloser
	SourceFilePath  string
	SourceStateFile string
	Outbox          *mpmc.Queue[ParsedMessage]
	Metrics         *MetricStorage
}

type JrnlInstance struct {
	Namespace []string
	Journal   io.ReadCloser
	StateFile string
	Outbox    *mpmc.Queue[ParsedMessage]
	Metrics   *MetricStorage
}

// Container for standard data/metadata
type ParsedMessage struct {
	Text            string
	ApplicationName string
	Hostname        string
	ProcessID       int
	Timestamp       time.Time
	Facility        string
	Severity        string
}
