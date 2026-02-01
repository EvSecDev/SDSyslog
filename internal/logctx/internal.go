package logctx

import (
	"sdsyslog/internal/global"
	"time"
)

// Logs event
func (logger *Logger) log(eventLevel int, eventSeverity string, tags []string, fullMessage string) {
	logger.mutex.Lock()
	currentLevel := logger.PrintLevel
	logger.mutex.Unlock()

	if eventLevel > currentLevel && eventSeverity != global.ErrorLog {
		return
	}

	event := Event{
		Timestamp: time.Now(),
		Tags:      tags,
		Severity:  eventSeverity,
		Message:   fullMessage,
	}

	logger.mutex.Lock()
	logger.queue = append(logger.queue, event)
	logger.cond.Signal() // Notify watcher that new event is available
	logger.mutex.Unlock()
}
