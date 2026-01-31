// Manages reader worker instances
package ingest

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new instance manager
func NewInstanceManager(ctx context.Context, outbox *mpmc.Queue[protocol.Message]) (new *InstanceManager) {
	// Double check queues - should never get past build
	if outbox == nil {
		panic("FATAL: Sender Ingest manager received empty outbox queue variable")
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, global.NSmIngest)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	new = &InstanceManager{
		FileSources: make(map[string]*FileWorker),
		outQueue:    outbox,
		ctx:         ctx,
	}
	return
}
