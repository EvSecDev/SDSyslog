package listener

import (
	"context"
	"sdsyslog/internal/queue/mpmc"
	"time"
)

// Push to mpmc parsed queue in a blocking manner (poll based)
func pushBlocking(ctx context.Context, queue *mpmc.Queue[ParsedMessage], msg ParsedMessage) {
	size := len(msg.Text) +
		len(msg.ApplicationName) +
		len(msg.Hostname) +
		len(msg.Facility) +
		len(msg.Severity) +
		16 // int64 size pid and time
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if queue.Push(msg) { // try once
				queue.ActiveWrite.Load().Metrics.Bytes.Add(uint64(size))
				return
			}
			time.Sleep(10 * time.Millisecond) // or backoff
		}
	}
}
