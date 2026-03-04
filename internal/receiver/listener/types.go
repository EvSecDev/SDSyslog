package listener

import (
	"context"
	"net"
	"sdsyslog/internal/queue/mpmc"
	"sync"
	"sync/atomic"
	"time"
)

type ManagerConfig struct {
	MinInstanceCount       atomic.Uint32 // Minimum number of instances at any one time
	MaxInstanceCount       atomic.Uint32 // Maximum number of instances at any one time
	Port                   int           // Network listen port
	ReplayProtectionWindow time.Duration // +/- time duration from packet reception time where a duplicate public key will cause packet to be dropped
	replayCleanInterval    time.Duration // Eviction check interval for seen public keys
}

type Manager struct {
	Config      *ManagerConfig              // Configuration values
	Instances   atomic.Pointer[[]*Instance] // Existing running instances
	replayCache *replayCache
	outbox      *mpmc.Queue[Container]
	ctx         context.Context
}

type Instance struct {
	conn       *net.UDPConn
	outbox     *mpmc.Queue[Container]
	minLen     int
	Metrics    MetricStorage
	isReplayed func(pubKey []byte) (replayed bool)

	ctx    context.Context
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
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

// For SPSC queue
type Container struct {
	Data []byte
	Meta Metadata
}
type Metadata struct {
	RemoteIP string
}
