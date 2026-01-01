package output

import (
	"io"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type Instance struct {
	Namespace []string
	FileOut   io.WriteCloser
	JrnlOut   io.WriteCloser
	Inbox     *mpmc.Queue[protocol.Payload]
	Metrics   *MetricStorage
}
