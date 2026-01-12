package listener

import (
	"net"
	"sdsyslog/internal/queue/mpmc"
)

type Instance struct {
	Namespace []string
	conn      *net.UDPConn
	Outbox    *mpmc.Queue[Container]
	minLen    int
	Metrics   MetricStorage
}

// For SPSC queue
type Container struct {
	Data []byte
	Meta Metadata
}
type Metadata struct {
	RemoteIP string
}
