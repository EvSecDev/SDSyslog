package out

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/output"
	"sdsyslog/pkg/protocol"
	"sync"
)

type ManagerConfig struct {
	MinQueueCapacity int // Minimum queue size (also starting size)
	MaxQueueCapacity int // Maximum queue size
}

type Manager struct {
	Config *ManagerConfig
	Queue  *mpmc.Queue[protocol.Payload] // Shared queue across all assembler/output instances

	Instance output.Instance    // Output worker writing to all configured outputs
	wg       sync.WaitGroup     // Waiter for instance
	cancel   context.CancelFunc // Stop instance

	ctx context.Context
}
