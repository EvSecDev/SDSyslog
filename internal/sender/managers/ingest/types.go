package ingest

import (
	"context"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
	"sync"
)

type InstanceManager struct {
	Mu            sync.Mutex
	FileSources   map[string]*FileWorker // File sources keyed by path
	JournalSource *JrnlWorker
	outQueue      *mpmc.Queue[global.ParsedMessage] // Queue for worked completed by the pair
	ctx           context.Context
}

type FileWorker struct {
	Worker *file.InModule
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
}

type JrnlWorker struct {
	Worker *journald.InModule
	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
}
