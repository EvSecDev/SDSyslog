package output

import (
	"context"
	"sdsyslog/internal/externalio/beats"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

type ManagerConfig struct {
	MinQueueCapacity int // Minimum queue size (also starting size)
	MaxQueueCapacity int // Maximum queue size
}

type Manager struct {
	Config *ManagerConfig
	Inbox  *mpmc.Queue[protocol.Payload] // Shared queue across all assembler/output instances

	Instance Instance           // Output worker writing to all configured outputs
	wg       sync.WaitGroup     // Waiter for instance
	cancel   context.CancelFunc // Stop instance

	ctx context.Context
}

type Instance struct {
	namespace []string
	fileMod   *file.OutModule
	jrnlMod   *journald.OutModule
	beatsMod  *beats.OutModule
	inbox     *mpmc.Queue[protocol.Payload]
	Metrics   MetricStorage
}
