package packaging

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/assembler"
	"sdsyslog/internal/sender/listener"
	"sync"
)

type InstanceManager struct {
	Mu             sync.Mutex                          // For adding/removing worker operations
	nextID         int                                 // Next unused output worker id
	Instances      map[int]*Instance                   // Individual output workers
	MinInstCount   int                                 // Minimum number of instances at any one time
	MaxInstCount   int                                 // Maximum number of instances at any one time
	InQueue        *mpmc.Queue[listener.ParsedMessage] // Messages from source processors
	outQueue       *mpmc.Queue[[]byte]                 // Queued packets to be sent
	hostID         int                                 // ID for all sent messages
	maxPayloadSize int                                 // maximum payload size for configured destination
	ctx            context.Context
}

type Instance struct {
	id     int                 // Manager map id
	Worker *assembler.Instance // Individual output worker
	wg     sync.WaitGroup      // Waiter for instance
	cancel context.CancelFunc  // Stop instance
}
