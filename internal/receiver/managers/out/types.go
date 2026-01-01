package out

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/output"
	"sdsyslog/pkg/protocol"
	"sync"
)

type InstanceManager struct {
	Queue    *mpmc.Queue[protocol.Payload] // Shared queue across all assembler/output instances
	Instance *OutputInstance               // Worker for writing output
	ctx      context.Context
}

type OutputInstance struct {
	Worker *output.Instance   // Individual output worker
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
