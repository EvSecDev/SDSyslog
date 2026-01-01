// Manages output writer worker instance
package out

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new instance manager with shared queue (between assemblers and output workers)
func NewInstanceManager(ctx context.Context, size int) (new *InstanceManager, err error) {
	// Add log context
	ctx = logctx.AppendCtxTag(ctx, global.NSmOutput)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	outQueue, err := mpmc.New[protocol.Payload](logctx.GetTagList(ctx), uint64(size), 2, global.DefaultMaxQueueSize)
	if err != nil {
		return
	}

	new = &InstanceManager{
		Instance: &OutputInstance{},
		Queue:    outQueue,
		ctx:      ctx,
	}
	return
}
