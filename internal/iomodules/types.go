package iomodules

import (
	"context"
	"sdsyslog/internal/metrics"
	"sdsyslog/pkg/protocol"
	"time"
)

// This package represents the different types of inputs and outputs from the core pipeline.
// All sub-packages in this package (iomodules) should implement these methods.

// Output Module Methods - For sending messages from the receive daemon pipeline to external sources
type Output interface {
	Write(ctx context.Context, msg *protocol.Payload) (entriesWritten int, err error) // Sends a message to the output
	FlushBuffer() (flushedCnt int, err error)                                         // For batching - flush current buffer to source immediately
	Shutdown() (err error)                                                            // Gracefully stops writer (for cleaning up resources)
}

// Input Module Methods - For reading messages into the send daemon pipeline
type Input interface {
	Start() (err error)                                                  // Starts reader
	Shutdown() (err error)                                               // Gracefully stops reader
	CollectMetrics(interval time.Duration) (collection []metrics.Metric) // Collects any domain-specific metrics within the given past interval
}
