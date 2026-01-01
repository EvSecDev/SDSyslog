package proc

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/processor"
	"sdsyslog/internal/receiver/shard"
	"sync"
)

type InstanceManager struct {
	Mu           sync.Mutex        // For scaling operations
	nextID       int               // Next free ID for new pair
	Instances    map[int]*Instance // Existing running
	MinInstCount int               // Minimum number of instances at any one time
	MaxInstCount int               // Maximum number of instances at any one time
	Inbox        *mpmc.Queue[listener.Container]
	routingView  shard.RoutingView // Allows processor to route to shards
	ctx          context.Context
}

type Instance struct {
	Processor *processor.Instance // Network packet parser

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
