package processor

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
	"sync"
	"sync/atomic"
	"time"
)

type ManagerConfig struct {
	MinQueueCapacity int           // Minimum queue size (also starting size)
	MaxQueueCapacity int           // Maximum queue size
	MinInstanceCount atomic.Uint32 // Minimum number of instances at any one time
	MaxInstanceCount atomic.Uint32 // Maximum number of instances at any one time
	PastMsgCutoff    time.Duration // Oldest time in the past messages can have
	FutureMsgCutoff  time.Duration // Max time in the future messages can have
}

type Manager struct {
	Config      *ManagerConfig // Configuration values
	Instances   atomic.Pointer[[]*Instance]
	Inbox       *mpmc.Queue[listener.Container]
	routingView shard.RoutingView // Allows processor to route to shards
	ctx         context.Context
}

type Instance struct {
	pastTimestampLimit   time.Duration
	futureTimestampLimit time.Duration
	inbox                *mpmc.Queue[listener.Container]
	routingView          shard.RoutingView
	Metrics              MetricStorage

	ctx    context.Context
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
