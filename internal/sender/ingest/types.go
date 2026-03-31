package ingest

import (
	"context"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"sync"
)

type ManagerConfig struct {
	SourceDropFilters map[string][]protocol.MessageFilter
}

type Manager struct {
	Config        *ManagerConfig
	FileSourceMu  sync.RWMutex
	FileSources   map[string]iomodules.Input // File sources keyed by path
	JournalSource iomodules.Input
	outQueue      *mpmc.Queue[protocol.Message] // Queue for worked completed by the pair
	ctx           context.Context
}
