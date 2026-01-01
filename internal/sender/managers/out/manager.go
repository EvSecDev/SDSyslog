// Manages output writer worker instance
package out

import (
	"context"
	"net"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
)

// Creates new instance manager (and its own inbox)
func NewInstanceManager(ctx context.Context, inboxSize int, conn *net.UDPConn, minInsts, maxInsts, minQsize, maxQsize int) (new *InstanceManager, err error) {
	// Add log context
	ctx = logctx.AppendCtxTag(ctx, global.NSmOutput)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	inQueue, err := mpmc.New[[]byte](logctx.GetTagList(ctx), uint64(inboxSize), minQsize, maxQsize)
	if err != nil {
		return
	}

	new = &InstanceManager{
		Instances:    make(map[int]*Instance),
		MinInstCount: minInsts,
		MaxInstCount: maxInsts,
		InQueue:      inQueue,
		OutDest:      conn,
		ctx:          ctx,
	}
	return
}
