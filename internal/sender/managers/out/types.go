package out

import (
	"context"
	"net"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/output"
	"sync"
	"sync/atomic"
)

type ManagerConfig struct {
	MinQueueCapacity int           // Minimum queue size (also starting size)
	MaxQueueCapacity int           // Maximum queue size
	MinInstanceCount atomic.Uint32 // Minimum number of instances at any one time
	MaxInstanceCount atomic.Uint32 // Maximum number of instances at any one time
	DestinationIP    string
	DestinationPort  int
}

type Manager struct {
	Config    *ManagerConfig
	Mu        sync.RWMutex        // For adding/removing worker operations
	nextID    int                 // Next unused output worker id
	Instances map[int]*Instance   // Individual output workers
	InQueue   *mpmc.Queue[[]byte] // Shared inbox for all workers
	outDest   *net.UDPConn        // Destination for all workers
	ctx       context.Context
}

type Instance struct {
	id     int                // manager map id
	Worker *output.Instance   // Individual output worker
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
