package out

import (
	"context"
	"net"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/output"
	"sync"
)

type InstanceManager struct {
	Mu           sync.Mutex          // For adding/removing worker operations
	nextID       int                 // Next unused output worker id
	Instances    map[int]*Instance   // Individual output workers
	MinInstCount int                 // Minimum number of instances at any one time
	MaxInstCount int                 // Maximum number of instances at any one time
	InQueue      *mpmc.Queue[[]byte] // Shared inbox for all workers
	OutDest      *net.UDPConn        // Destination for all workers
	ctx          context.Context
}

type Instance struct {
	id     int                // manager map id
	Worker *output.Instance   // Individual output worker
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
