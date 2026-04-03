package journald

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

type OutModule struct {
	sink   *http.Client
	url    string
	bootID string
}

type InModule struct {
	// Settings
	filters       []protocol.MessageFilter
	localHostname string

	// Inputs
	cmd  *exec.Cmd
	sink io.ReadCloser

	readerReady chan struct{}
	readyOnce   sync.Once

	err io.ReadCloser // stderr pipe from journalctl command

	// Output
	outbox *mpmc.Queue[*protocol.Message]

	// State
	stateFile string

	metrics MetricStorage

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
	ctx    context.Context
}
