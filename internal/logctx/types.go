package logctx

import (
	"sync"
	"time"
)

// Log Event Structure
type Event struct {
	Timestamp time.Time
	Severity  string
	Tags      []string
	Message   string
}

// Logger Struct
type Logger struct {
	ID         string
	CreatedAt  time.Time
	queue      []Event // event buffer
	mutex      sync.Mutex     // protects buffer
	cond       *sync.Cond     // condition to signal new events
	Done       <-chan struct{}
	PrintLevel int             // Level at which the message should be recorded
	wg         *sync.WaitGroup // Holds main execution threads until log watchers are done handling events
}
