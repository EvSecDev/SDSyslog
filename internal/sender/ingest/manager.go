// Manages reader worker instances
package ingest

import (
	"context"
	"fmt"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new instance manager
func (config *ManagerConfig) NewManager(ctx context.Context, outbox *mpmc.Queue[*protocol.Message]) (new *Manager, err error) {
	// Double check queues - should never get past build
	if outbox == nil {
		err = fmt.Errorf("ingest manager received empty outbox queue variable")
		return
	}

	// Add log context
	ctx = logctx.AppendCtxTag(ctx, logctx.NSmIngest)
	defer func() { ctx = logctx.RemoveLastCtxTag(ctx) }()

	new = &Manager{
		Config:      config,
		FileSources: make(map[string]iomodules.Input),
		outQueue:    outbox,
		ctx:         ctx,
	}
	return
}
