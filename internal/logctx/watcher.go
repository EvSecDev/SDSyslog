package logctx

import (
	"fmt"
	"io"
	"sdsyslog/internal/global"
	"strings"
	"time"
)

// Hold main thread exit until logger is finished its work
func (logger *Logger) Wait() {
	logger.wg.Wait()
}

// Wake signals/broadcasts to any goroutines waiting on the condition variable
func (logger *Logger) Wake() {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()
	logger.cond.Broadcast()
}

// Starts a go routine that reads events and writes formatted output to io.Writer.
// Stops when logger.Done is closed.
func StartWatcher(logger *Logger, output io.Writer) {
	logger.wg.Add(1)

	go func() {
		defer logger.wg.Done()

		var dedup dedupState
		const dedupWindow = 5 * time.Second
		const minRepeats = 10
		const suppressCooldown = 1 * time.Minute

		for {
			logger.mutex.Lock()

			// If done and queue is empty, exit
			if len(logger.queue) == 0 {
				select {
				case <-logger.Done:
					logger.mutex.Unlock()
					return
				default:
				}
			}

			// Wait for events
			for len(logger.queue) == 0 {
				select {
				case <-logger.Done:
					logger.mutex.Unlock()
					return
				default:
					logger.cond.Wait()
				}
			}

			// Pop one event from the front of the queue
			event := logger.queue[0]
			logger.queue = logger.queue[1:]
			logger.mutex.Unlock()

			now := time.Now()

			// Deduplication logic
			// Duplicate events older than the deduplication window are not considered duplicates.
			// Purely meant for highly repetitive message suppression to prevent excessive noise.
			if event.Message != "" &&
				event.Message == dedup.lastMsg &&
				now.Sub(event.Timestamp) <= dedupWindow {

				dedup.repeatCount++
				// Only print suppression message once per minute
				if dedup.repeatCount >= minRepeats && now.Sub(dedup.lastSuppressTime) >= suppressCooldown {

					fmt.Fprintf(output,
						"[%s] [%s] [%s] Suppressed %d repeated messages: %s\n",
						padTimestamp(event.Timestamp),
						strings.Join(event.Tags, "/"),
						global.InfoLog,
						dedup.repeatCount,
						dedup.lastMsg)

					dedup.lastSuppressTime = now
					dedup.repeatCount = 0
				}

				// skip printing this repeated message
				continue
			} else {
				// Reset counter if message changes or window exceeded
				dedup.lastMsg = event.Message
				dedup.repeatCount = 1
			}

			fmt.Fprintf(output, "%s", event.Format())
		}
	}()
}
