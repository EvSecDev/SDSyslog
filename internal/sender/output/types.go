package output

import (
	"net"
	"sdsyslog/internal/queue/mpmc"
)

type Instance struct {
	Namespace []string
	inbox     *mpmc.Queue[[]byte]
	conn      *net.UDPConn
	Metrics   *MetricStorage
}
