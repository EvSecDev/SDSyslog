// Fake IO Module for injecting internal log events into pipeline. Does NOT satisfy io module interface - meant for specific daemon use only
package internallogger

import (
	"context"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

type SenderInjector struct {
	logger *logctx.Logger

	// Assembler-Owned Queue (Ingest->Assembler queue)
	outbox *mpmc.Queue[*protocol.Message]

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
	ctx    context.Context
}

type ReceiverInjector struct {
	logger *logctx.Logger

	hostID int

	// Output-Owned Queue (Assembler->Output queue)
	outbox *mpmc.Queue[*protocol.Payload]

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
	ctx    context.Context
}
