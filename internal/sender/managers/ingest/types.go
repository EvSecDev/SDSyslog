package ingest

import (
	"context"
	"os/exec"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/internal/sender/listener"
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
	Worker *listener.FileInstance // reader+Parser

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
}

type JrnlWorker struct {
	Worker  *listener.JrnlInstance // reader+Parser
	Command *exec.Cmd

	wg     sync.WaitGroup     // Waiter for instance
	cancel context.CancelFunc // cancel instance
}
