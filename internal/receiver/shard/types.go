package shard

import (
	"sdsyslog/pkg/protocol"
	"sync"
	"sync/atomic"
	"time"
)

type Bucket struct {
	filled               bool                     // Marker for done bucket awaiting assembly
	Fragments            map[int]protocol.Payload // keyed by sequence number
	maxSeq               int                      // max sequence number expected
	lastProcessStartTime time.Time                // when processor last started processing a fragment
}

type Instance struct {
	Namespace      []string
	Mu             sync.Mutex
	Buckets        map[string]*Bucket // keyed by bucket ID
	KeyQueue       chan string        // FIFO of filled bucket keys
	PacketDeadline *atomic.Int64      // Owned by manager
	InShutdown     bool               // Blocks new bucket creation
	Metrics        MetricStorage
}

// Interface for making fragment routing decisions
type RoutingView interface {
	GetShardCount() int
	GetShard(index int) *Instance
	IsShardShutdown(index int) bool
	BucketExists(index int, bucketKey string) bool

	GetOverride(bucketKey string) (int, bool)
	SetOverride(bucketKey string, index int)

	FindAlternativeShard(orig int) int
}

// Interface for cleaning up temporary fragment routing overrides
type OverrideCleaner interface {
	ClearOverride(bucketKey string)
}
