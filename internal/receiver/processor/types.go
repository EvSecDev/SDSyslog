package processor

import (
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
)

type Instance struct {
	Namespace   []string
	inbox       *mpmc.Queue[listener.Container]
	routingView shard.RoutingView
	Metrics     MetricStorage
}
