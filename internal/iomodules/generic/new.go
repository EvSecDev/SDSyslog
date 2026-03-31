// Generic in/out module that takes the reader and writer from the caller. Only writes payload data, no fields or other metadata
package generic

import (
	"context"
	"fmt"
	"io"
	"os"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new send pipeline input with the given source
func NewInput(ctx context.Context, source io.ReadCloser, queue *mpmc.Queue[protocol.Message]) (new *InModule, err error) {
	ctx, cancel := context.WithCancel(ctx)
	newNamespace := append(logctx.GetTagList(ctx), logctx.NSoRaw)
	ctx = logctx.OverwriteCtxTag(ctx, newNamespace)

	new = &InModule{
		sink:    source,
		outbox:  queue,
		metrics: MetricStorage{},
		ctx:     ctx,
		cancel:  cancel,
	}

	new.localHostname, err = os.Hostname()
	if err != nil {
		err = fmt.Errorf("failed to retrieve local hostname: %w", err)
		return
	}
	return
}

// Creates new receiver pipeline output with the given destination
func NewOutput(destination io.WriteCloser, batchSize int) (new *OutModule) {
	if destination == nil {
		return
	}

	if batchSize == 0 {
		batchSize = 20
	}

	new = &OutModule{
		sink:      destination,
		batchSize: batchSize,
		buffer:    make([]protocol.Payload, 0, batchSize),
	}
	return
}
