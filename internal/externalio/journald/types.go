package journald

import (
	"io"
	"net/http"
	"os/exec"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
)

type OutModule struct {
	sink *http.Client
	url  string
}

type InModule struct {
	Namespace []string
	cmd       *exec.Cmd
	sink      io.ReadCloser
	err       io.ReadCloser
	stateFile string
	outbox    *mpmc.Queue[global.ParsedMessage]
	metrics   MetricStorage
}
