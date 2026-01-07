package output

import (
	"io"
	"net/http"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type Instance struct {
	Namespace []string
	FileOut   io.WriteCloser
	JrnlOut   *http.Client
	JrnlURL   string
	Inbox     *mpmc.Queue[protocol.Payload]
	Metrics   *MetricStorage
}
