// Manages assembler, shard, and deadline evaluator worker instances
package defrag

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Create new defrag manager
func NewInstanceManager(ctx context.Context, outbox *mpmc.Queue[protocol.Payload], minInsts, maxInsts int) (new *InstanceManager) {
	// Double check queues - should never get past build
	if outbox == nil {
		panic("FATAL: Receiver Defrag manager received empty outbox queue variable")
	}

	ctx = logctx.AppendCtxTag(ctx, logctx.NSmDefrag)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	startRouteSnapshot := routingSnapshot{
		pairs: make(map[string]*InstancePair, minInsts),
		ids:   make([]string, minInsts),
	}

	new = &InstanceManager{
		minInstCount: minInsts,
		maxInstCount: maxInsts,
		outQueue:     outbox,
		ctx:          ctx,
	}

	new.PacketDeadline.Store(int64(global.DefaultMinPacketDeadline))
	new.routing.Store(&startRouteSnapshot)
	new.RoutingView = &RoutingState{
		manager: new,
	}
	return
}

// Configured (static) minimum instance count
func (manager *InstanceManager) GetMinimumInstances() (min int) {
	min = manager.minInstCount
	return
}

// Configured (static) maximum instance count
func (manager *InstanceManager) GetMaximumInstances() (max int) {
	max = manager.maxInstCount
	return
}
