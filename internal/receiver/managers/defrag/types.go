package defrag

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
	"sync"
	"sync/atomic"
)

type ManagerConfig struct {
	MinInstanceCount atomic.Uint32 // Minimum number of instances at any one time
	MaxInstanceCount atomic.Uint32 // Maximum number of instances at any one time
	PacketDeadline   atomic.Int64  // Manager owns this value
}

type Manager struct {
	Config         *ManagerConfig                  // Configuration values
	scalingMutex   sync.RWMutex                    // Serializes add/remove - scaling operations are single-threaded
	nextInstanceID uint16                          // Next instance pair ID
	routing        atomic.Pointer[routingSnapshot] // Atomic pointer to immutable routing snapshot used by hot-path readers
	RoutingView    *RoutingState                   // External read-only by method for viewing routing - prevents direct manager access and import cycles
	outQueue       *mpmc.Queue[protocol.Payload]   // Next pipeline stage queue (not owned by this manager)
	FIPRRunning    atomic.Bool                     // Syncs fipr send to local fipr receive to gate hot path from checking socket directory unnecessarily
	ctx            context.Context
}

type routingSnapshot struct {
	pairs map[string]*InstancePair
	ids   []string // FIFO pool of IDs for routing (also used as sliding window for wraparound mitigation)
}

type InstancePair struct {
	Shard     *shard.Instance     // Fragment container and watcher
	Assembler *assembler.Instance // Packet re-assembler

	wg     sync.WaitGroup     // Waiter for both instances
	cancel context.CancelFunc // Shared cancel (stops both pairs)
}

type RoutingState struct {
	manager *Manager
}
