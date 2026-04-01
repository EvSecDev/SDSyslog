package generic

import (
	"context"
	"io"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

type InModule struct {
	localHostname string

	sink   io.ReadCloser
	outbox *mpmc.Queue[protocol.Message]

	metrics MetricStorage

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
	ctx    context.Context
}

type OutModule struct {
	sink io.WriteCloser
}
