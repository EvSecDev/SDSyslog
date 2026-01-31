// Manages assembler worker instances
package packaging

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new instance manager
func NewInstanceManager(ctx context.Context, inboxSize int, outbox *mpmc.Queue[[]byte], hostID, maxPayloadSize, minInsts, maxInsts, minQsize, maxQsize int) (new *InstanceManager, err error) {
	// Double check queue - should never get past build
	if outbox == nil {
		panic("FATAL: Sender Packaging manager received empty outbox queue variable")
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, global.NSmPack)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	inbox, err := mpmc.New[protocol.Message](logctx.GetTagList(ctx), uint64(inboxSize), minQsize, maxQsize)
	if err != nil {
		return
	}

	new = &InstanceManager{
		Instances:      make(map[int]*Instance),
		MinInstCount:   minInsts,
		MaxInstCount:   maxInsts,
		InQueue:        inbox,
		outQueue:       outbox,
		hostID:         hostID,
		maxPayloadSize: maxPayloadSize,
		ctx:            ctx,
	}
	return
}
