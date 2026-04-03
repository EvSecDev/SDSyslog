package shard

import (
	"sdsyslog/pkg/protocol"
	"sync"
	"sync/atomic"
	"time"
)

type Bucket struct {
	filled               bool                     // Marker for done bucket awaiting assembly
	Fragments            map[int]*protocol.Payload // keyed by sequence number
	maxSeq               int                      // max sequence number expected
	lastProcessStartTime time.Time                // when processor last started processing a fragment
}

type Instance struct {
	Namespace      []string
	Mu             sync.Mutex
	Buckets        map[string]*Bucket // keyed by bucket ID
	keyQueue       chan string        // FIFO of filled bucket keys
	packetDeadline *atomic.Int64      // Owned by manager
	InShutdown     atomic.Bool        // Blocks new bucket creation
	Metrics        MetricStorage
}

// Interface for making fragment routing decisions
// Source functions are located in the Defrag manager package at 'internal/receiver/assembler/routing.go'
type RoutingView interface {
	GetAllIDs() (shardIDs []string)
	GetNonDrainingIDs() (availShardIDs []string)
	BucketExists(shardID string, bucketKey string) (present bool)
	GetShard(shardID string) (shardInst *Instance)
	IsShardShutdown(shardID string) (shutdown bool)
	BucketExistsAnywhere(bucketKey string) (present bool)
	IsFIPRRunning() (running bool)
	SocketDir() (path string)
}
