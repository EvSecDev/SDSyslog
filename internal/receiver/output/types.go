package output

import (
	"context"
	"io"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

type ManagerConfig struct {
	FilePath         string
	JournaldURL      string
	BeatsAddress     string
	RawWriter        io.WriteCloser
	EnableDBUSNotify bool

	MinQueueCapacity global.MinValue // Minimum queue size (also starting size)
	MaxQueueCapacity global.MaxValue // Maximum queue size
}

type Manager struct {
	Config *ManagerConfig
	Inbox  *mpmc.Queue[*protocol.Payload] // Shared queue across all assembler/output instances

	Instance Instance           // Output worker writing to all configured outputs
	wg       sync.WaitGroup     // Waiter for instance
	cancel   context.CancelFunc // Stop instance

	ctx context.Context
}

type Instance struct {
	namespace  []string
	fileMod    iomodules.Output
	jrnlMod    iomodules.Output
	beatsMod   iomodules.Output
	rawMod     iomodules.Output
	DBUSnotify iomodules.Output

	inbox   *mpmc.Queue[*protocol.Payload]
	Metrics MetricStorage
}
