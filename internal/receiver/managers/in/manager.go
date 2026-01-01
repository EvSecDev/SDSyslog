// Manages packet listener worker instances
package in

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/receiver/listener"
)

// Creates new instance manager
func NewInstanceManager(ctx context.Context, port int, outQueue *mpmc.Queue[listener.Container], minInsts, maxInsts int) (new *InstanceManager) {
	ctx = logctx.AppendCtxTag(ctx, global.NSmIngest)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	new = &InstanceManager{
		Instances:    make(map[int]*Instance),
		MinInstCount: minInsts,
		MaxInstCount: maxInsts,
		port:         port,
		outbox:       outQueue,
		ctx:          ctx,
	}
	return
}
