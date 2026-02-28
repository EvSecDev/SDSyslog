package processor

import (
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
	"time"
)

type Instance struct {
	Namespace            []string
	pastTimestampLimit   time.Duration
	futureTimestampLimit time.Duration
	inbox                *mpmc.Queue[listener.Container]
	routingView          shard.RoutingView
	Metrics              MetricStorage
}
