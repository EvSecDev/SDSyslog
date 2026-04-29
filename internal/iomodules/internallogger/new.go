package internallogger

import (
	"context"
	"fmt"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

// New injector for the Sender daemon. Used for writing log events directly into destination queue.
// Should take Ingest->Assembler queue as destination.
func NewSenderInjector(ctx context.Context, destinationQueue *mpmc.Queue[*protocol.Message]) (injector *SenderInjector, err error) {
	if destinationQueue == nil {
		err = fmt.Errorf("destination queue is nil")
		return
	}

	ctxLogger := logctx.GetLogger(ctx)
	if ctxLogger == nil {
		err = fmt.Errorf("logger in context is nil")
		return
	}

	newNamespace := append(logctx.GetTagList(ctx), logctx.NSLogger)
	modCtx := logctx.OverwriteCtxTag(ctx, newNamespace)
	modCtx, cancel := context.WithCancel(modCtx)

	injector = &SenderInjector{
		logger: ctxLogger,

		outbox: destinationQueue,

		wg:     sync.WaitGroup{},
		ctx:    modCtx,
		cancel: cancel,
	}
	return
}

// New injector for the Receiver daemon. Used for writing log events directly into destination queue.
// Should take Assembler->Output queue as destination.
func NewReceiverInjector(ctx context.Context, destinationQueue *mpmc.Queue[*protocol.Payload], hostID int) (injector *ReceiverInjector, err error) {
	if destinationQueue == nil {
		err = fmt.Errorf("destination queue is nil")
		return
	}

	ctxLogger := logctx.GetLogger(ctx)
	if ctxLogger == nil {
		err = fmt.Errorf("logger in context is nil")
		return
	}

	newNamespace := append(logctx.GetTagList(ctx), logctx.NSLogger)
	modCtx := logctx.OverwriteCtxTag(ctx, newNamespace)
	modCtx, cancel := context.WithCancel(modCtx)

	injector = &ReceiverInjector{
		logger: ctxLogger,

		hostID: hostID,

		outbox: destinationQueue,

		wg:     sync.WaitGroup{},
		ctx:    modCtx,
		cancel: cancel,
	}
	return
}
