package assembler

import (
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/protocol"
)

type Instance struct {
	Namespace []string
	shardInst *shard.Instance
	outbox    *mpmc.Queue[protocol.Payload]
	cleaner   shard.OverrideCleaner
	Metrics   MetricStorage
}
