package in

import (
	"context"
	"net"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sync"
)

type InstanceManager struct {
	Mu           sync.Mutex        // For scaling operations
	nextID       int               // Next free ID for new pair
	Instances    map[int]*Instance // Existing running instances
	MinInstCount int               // Minimum number of instances at any one time
	MaxInstCount int               // Maximum number of instances at any one time
	port         int               // Network listen port
	outbox       *mpmc.Queue[listener.Container]
	ctx          context.Context
}

type Instance struct {
	Listener *listener.Instance // Network packet reader
	conn     *net.UDPConn       // Socket (reused) for the listener

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // Stop instance
}
