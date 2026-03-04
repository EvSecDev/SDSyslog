package assembler

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
	"sync/atomic"
)

type ManagerConfig struct {
	MinQueueCapacity       int           // Minimum queue size (also starting size)
	MaxQueueCapacity       int           // Maximum queue size
	MinInstanceCount       atomic.Uint32 // Minimum number of instances at any one time
	MaxInstanceCount       atomic.Uint32 // Maximum number of instances at any one time
	HostID                 int           // ID for all sent messages
	DestinationIP          string        // Destination address for output
	OverrideMaxPayloadSize int           // Use supplied maximum payload size
	MaxPayloadSize         int           // Maximum payload size for configured destination
}

type Manager struct {
	Config    *ManagerConfig                // Configuration Values
	Instances atomic.Pointer[[]*Instance]   // Individual output workers
	InQueue   *mpmc.Queue[protocol.Message] // Messages from source processors
	outQueue  *mpmc.Queue[[]byte]           // Queued packets to be sent
	ctx       context.Context
}

type Instance struct {
	inbox          *mpmc.Queue[protocol.Message] // messages from processors
	outbox         *mpmc.Queue[[]byte]           // fragments for sender
	hostID         int                           // ID for all sent messages
	maxPayloadSize int                           // maximum payload size for configured destination
	Metrics        MetricStorage

	ctx    context.Context
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
