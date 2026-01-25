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

type InstanceManager struct {
	Mu             sync.RWMutex // For scaling operations
	InstancePairs  []*InstancePair
	MinInstCount   int // Minimum number of instances at any one time
	MaxInstCount   int // Maximum number of instances at any one time
	outQueue       *mpmc.Queue[protocol.Payload]
	PacketDeadline atomic.Int64 // Manager owns this value
	Routing        *RoutingState
	ctx            context.Context
}

type InstancePair struct {
	Shard     *shard.Instance     // Fragment container and watcher
	Assembler *assembler.Instance // Packet re-assembler

	wg     sync.WaitGroup     // Waiter for both instances
	cancel context.CancelFunc // Shared cancel (stops both pairs)
}

type RoutingState struct {
	Mu sync.Mutex

	Manager   *InstanceManager
	Overrides map[string]int
}
