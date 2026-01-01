package logctx

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Starts a go routine that reads events and writes formatted output to io.Writer.
// Stops when logger.Done is closed.
func StartWatcher(logger *Logger, output io.Writer) {
	logger.wg.Add(1)

	go func() {
		defer logger.wg.Done()

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

			// Only print parts that are present
			var parts []string
			if !event.Timestamp.IsZero() {
				parts = append(parts, fmt.Sprintf("[%s]", padTimestamp(event.Timestamp)))
			}

			if len(event.Tags) > 0 {
				tagPrefixes := "["
				tagPrefixes += strings.Join(event.Tags, "/")
				tagPrefixes += "]"
				parts = append(parts, tagPrefixes)
			}

			if event.Severity != "" {
				parts = append(parts, fmt.Sprintf("[%s]", event.Severity))
			}

			if event.Message != "" {
				parts = append(parts, event.Message)
			}

			// printf to allow caller to determine newlines
			fmt.Fprintf(output, "%s", strings.Join(parts, " "))
		}
	}()
}

func (logger *Logger) GetFormattedLogLines() (formatted []string) {
	// Copy under lock to avoid holding mutex while sorting/formatting
	logger.mutex.Lock()
	events := make([]Event, len(logger.queue))
	copy(events, logger.queue)
	logger.mutex.Unlock()

	// Stable sort: oldest to newest
	sort.SliceStable(events, func(i, j int) bool {
		ti := events[i].Timestamp
		tj := events[j].Timestamp

		// Zero timestamps sort last
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})

	formatted = make([]string, 0, len(logger.queue))
	for _, event := range events {
		var parts []string

		// Message timestamp
		if !event.Timestamp.IsZero() {
			parts = append(parts, fmt.Sprintf("[%s]", padTimestamp(event.Timestamp)))
		}

		// Message tags
		if len(event.Tags) > 0 {
			tagPrefixes := "["
			tagPrefixes += strings.Join(event.Tags, "/")
			tagPrefixes += "]"
			parts = append(parts, tagPrefixes)
		}

		// Message severity
		if event.Severity != "" {
			parts = append(parts, fmt.Sprintf("[%s]", event.Severity))
		}

		// Main Text
		if event.Message != "" {
			msg := event.Message

			// Append newlines if not present
			if !strings.HasSuffix(msg, "\n") {
				msg += "\n"
			}

			parts = append(parts, msg)
		}

		// Final string
		formatted = append(formatted, strings.Join(parts, " "))
	}
	return
}

// Ensures fixed length strings for timestamps
func padTimestamp(timestamp time.Time) (formatted string) {
	formatted = timestamp.Format(time.RFC3339Nano)

	majorFields := strings.Split(formatted, ".")
	if len(majorFields) != 2 {
		return
	}

	minorFields := strings.Split(majorFields[1], "-")
	if len(minorFields) != 2 {
		return
	}

	tsPrefix := majorFields[0]
	nanoseconds := minorFields[0]
	timezoneOffset := minorFields[1]

	// Pad the nanoseconds part to ensure it's 9 digits long
	for len(nanoseconds) < 9 {
		nanoseconds = "0" + nanoseconds
	}

	// Rebuild the timestamp with padded nanoseconds and the original timezone offset
	formatted = fmt.Sprintf("%s.%s-%s", tsPrefix, nanoseconds, timezoneOffset)
	return
}
