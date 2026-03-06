package journald

import (
	"io"
	"net/http"
	"os/exec"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type OutModule struct {
	sink   *http.Client
	url    string
	bootID string
}

type InModule struct {
	Namespace []string
	cmd       *exec.Cmd
	sink      io.ReadCloser
	err       io.ReadCloser
	stateFile string
	filters   []protocol.MessageFilter
	outbox    *mpmc.Queue[protocol.Message]
	metrics   MetricStorage
}
