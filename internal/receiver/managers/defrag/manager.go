// Manages assembler, shard, and deadline evaluator worker instances
package defrag

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"time"
)

// Create new defrag manager
func NewInstanceManager(ctx context.Context, outbox *mpmc.Queue[protocol.Payload], minInsts, maxInsts int) (new *InstanceManager) {
	// Double check queues - should never get past build
	if outbox == nil {
		panic("FATAL: Receiver Defrag manager received empty outbox queue variable")
	}

	ctx = logctx.AppendCtxTag(ctx, global.NSmDefrag)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	new = &InstanceManager{
		InstancePairs: make([]*InstancePair, 0),
		MinInstCount:  maxInsts,
		MaxInstCount:  maxInsts,
		outQueue:      outbox,
		ctx:           ctx,
	}
	new.PacketDeadline.Store(int64(50 * time.Millisecond)) // Default starting deadline
	new.Routing = &RoutingState{
		Manager:   new,
		Overrides: make(map[string]int),
	}
	return
}
