// Central logging system. Buffers messages and writes to configured outputs
package logctx

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Entry for logging events.
// If event level is above the current set logger level, message will not be recorded.
// If severity is an error, event level is not considered and message is recorded.
func LogEvent(ctx context.Context, eventLevel int, severity string, message string, vars ...any) {
	// Retrieve current tag list
	tags := GetTagList(ctx)

	// Get logger pointer
	logger := GetLogger(ctx)
	if logger != nil {
		var newMsg string

		// vars might be empty - check to omit formatting
		if len(vars) == 0 || (!strings.Contains(message, "%") && !strings.Contains(message, `%%`)) {
			// Avoiding 'extra' print to log entries
			newMsg = message
		} else {
			// Maintain %w error wrapping compatibility
			message = strings.ReplaceAll(message, "%w", "%v")

			newMsg = fmt.Sprintf(message, vars...)
		}
		logger.log(eventLevel, severity, tags, newMsg)
	}
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

	formatted = make([]string, 0, len(events))
	for _, event := range events {
		msg := event.Format()

		// Append newlines if not present
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}

		// Final string
		formatted = append(formatted, msg)
	}
	return
}
