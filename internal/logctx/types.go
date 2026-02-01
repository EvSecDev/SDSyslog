package logctx

import (
	"sync"
	"time"
)

// Log Event Structure
type Event struct {
	Timestamp time.Time // Time when event enters log buffer
	Severity  string
	Tags      []string
	Message   string
}

// Logger Struct
type Logger struct {
	ID         string          // Unique identifier within the program
	CreatedAt  time.Time       // Time when logger was created
	queue      []Event         // event buffer
	mutex      sync.Mutex      // protects buffer
	cond       *sync.Cond      // condition to signal new events
	Done       <-chan struct{} // graceful shutdown (mainly for watcher)
	PrintLevel int             // Level at which the message should be recorded
	wg         *sync.WaitGroup // Holds main execution threads until log watchers are done handling events
}

// Tracks successive duplicate messages to prevent log flooding
type dedupState struct {
	lastMsg          string
	repeatCount      int
	lastSuppressTime time.Time
}
