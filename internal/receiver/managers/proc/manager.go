// Manages processor worker instances
package proc

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
)

// Creates new instance manager
func NewInstanceManager(ctx context.Context, inQueueSize int, shardRouting shard.RoutingView, minInsts, maxInsts int, minQsize, maxQsize int) (new *InstanceManager, err error) {
	// Add log context
	ctx = logctx.AppendCtxTag(ctx, global.NSProc)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	inQueue, err := mpmc.New[listener.Container](logctx.GetTagList(ctx), uint64(inQueueSize), minQsize, maxQsize)
	if err != nil {
		return
	}

	new = &InstanceManager{
		Instances:    make(map[int]*Instance),
		MinInstCount: minInsts,
		MaxInstCount: maxInsts,
		Inbox:        inQueue,
		routingView:  shardRouting,
		ctx:          ctx,
	}
	return
}
