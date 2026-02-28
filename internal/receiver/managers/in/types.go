package in

import (
	"context"
	"net"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sync"
	"time"
)

type InstanceManager struct {
	Mu                  sync.Mutex        // For scaling operations
	nextID              int               // Next free ID for new pair
	Instances           map[int]*Instance // Existing running instances
	MinInstCount        int               // Minimum number of instances at any one time
	MaxInstCount        int               // Maximum number of instances at any one time
	port                int               // Network listen port
	replayCleanInterval time.Duration
	replayCache         *replayCache
	outbox              *mpmc.Queue[listener.Container]
	ctx                 context.Context
}

// Replay attack protection for all listener instances
type replayCacheShard struct {
	mu    sync.Mutex
	store map[string]int64 // key -> unix timestamp
}

type replayCache struct {
	shards []*replayCacheShard
	ttl    int64 // seconds
}

type Instance struct {
	Listener *listener.Instance // Network packet reader
	conn     *net.UDPConn       // Socket (reused) for the listener

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
