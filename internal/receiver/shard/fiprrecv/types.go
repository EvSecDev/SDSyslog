package fiprrecv

import (
	"context"
	"net"
	"sdsyslog/internal/receiver/shard"
	"sync"
)

type Instance struct {
	Namespace        []string
	socketPath       string
	listener         net.Listener
	localRoutingView shard.RoutingView
	hmacSecret       []byte
	Metrics          MetricStorage

	wg     sync.WaitGroup     // Waiter for instance
	wgConn sync.WaitGroup     // Waiter for connections
	cancel context.CancelFunc // Stop instance
	ctx    context.Context
}
