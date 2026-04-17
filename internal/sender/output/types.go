package output

import (
	"context"
	"net"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
	"sync"
	"sync/atomic"
)

type ManagerConfig struct {
	MinQueueCapacity global.MinValue // Minimum queue size (also starting size)
	MaxQueueCapacity global.MaxValue // Maximum queue size
	MinInstanceCount atomic.Uint32   // Minimum number of instances at any one time
	MaxInstanceCount atomic.Uint32   // Maximum number of instances at any one time
	SourceAddress    *net.UDPAddr    // Source listen address
	DestAddress      *net.UDPAddr
}

type Manager struct {
	Config    *ManagerConfig
	Instances atomic.Pointer[[]*Instance] // Existing running instances
	InQueue   *mpmc.Queue[[]byte]         // Shared inbox for all workers
	outDest   *net.UDPConn                // Destination for all workers
	ctx       context.Context
}

type Instance struct {
	inbox   *mpmc.Queue[[]byte]
	conn    *net.UDPConn
	Metrics MetricStorage

	ctx    context.Context
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
