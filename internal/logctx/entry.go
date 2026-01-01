// Central logging system. Buffers messages and writes to configured outputs
package logctx

import (
	"context"
	"fmt"
	"sdsyslog/internal/global"
	"strings"
	"sync"
	"time"
)

// Logger Constructor
func NewLogger(id string, logLevel int, done <-chan struct{}) (logger *Logger) {
	logger = &Logger{
		ID:         id,
		CreatedAt:  time.Now(),
		queue:      make([]Event, 0),
		Done:       done,
		PrintLevel: logLevel,
		wg:         &sync.WaitGroup{},
	}
	logger.cond = sync.NewCond(&logger.mutex)
	return
}

// Attach the logger to context
func WithLogger(ctx context.Context, logger *Logger) (ctxLogger context.Context) {
	ctxLogger = context.WithValue(ctx, global.LoggerKey, logger)
	return
}

// Change the loggers level
func SetLogLevel(ctx context.Context, newLevel int) {
	logger := GetLogger(ctx)
	if logger != nil {
		logger.mutex.Lock()
		defer logger.mutex.Unlock()
		logger.PrintLevel = newLevel
	}
}

// Extracts Logger from context or returns nil
func GetLogger(ctx context.Context) (logger *Logger) {
	logger, ok := ctx.Value(global.LoggerKey).(*Logger)
	if ok {
		return
	}
	logger = nil
	return
}

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

// Entry for logging events
func LogEvent(ctx context.Context, eventLevel int, severity string, message string, vars ...any) {
	// Retrieve current tag list
	tags := GetTagList(ctx)

	// Get logger pointer
	logger := GetLogger(ctx)
	if logger != nil {
		var newMsg string

		// vars might be empty - check to omit formatting
		if vars == nil || !strings.Contains(message, "%") && !strings.Contains(message, `%%`) {
			// Avoiding 'extra' print to log entries
			newMsg = message
		} else {
			newMsg = fmt.Sprintf(message, vars...)
		}
		logger.log(eventLevel, severity, tags, newMsg)
	}
}
