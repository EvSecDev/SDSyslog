// Manages processor worker instances
package proc

import (
	"context"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
	"time"
)

// Creates new instance manager
func NewInstanceManager(ctx context.Context,
	shardRouting shard.RoutingView,
	minInsts, maxInsts int,
	minQsize, maxQsize int,
	oldMsgCutoff, futureMsgCutoff time.Duration) (new *InstanceManager, err error) {
	// Add log context
	ctx = logctx.AppendCtxTag(ctx, logctx.NSProc)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	inQueue, err := mpmc.New[listener.Container](logctx.GetTagList(ctx), uint64(minQsize), minQsize, maxQsize)
	if err != nil {
		return
	}

	new = &InstanceManager{
		Instances:       make(map[int]*Instance),
		MinInstCount:    minInsts,
		MaxInstCount:    maxInsts,
		pastMsgCutoff:   oldMsgCutoff,
		futureMsgCutoff: futureMsgCutoff,
		Inbox:           inQueue,
		routingView:     shardRouting,
		ctx:             ctx,
	}
	return
}
