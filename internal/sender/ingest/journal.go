package ingest

import (
	"fmt"
	"sdsyslog/internal/iomodules/journald"
	"time"
)

// Create journal ingest instance
func (manager *Manager) AddJrnlInstance(stateFile string, stateSaveInterval time.Duration) (err error) {
	if manager.JournalSource != nil {
		err = fmt.Errorf("cannot start a new journal instance with one running")
		return
	}

	filters := manager.Config.SourceDropFilters[JrnlSource]
	manager.JournalSource, err = journald.NewInput(manager.ctx, stateFile, stateSaveInterval, filters, manager.outQueue)
	if err != nil {
		return
	}

	err = manager.JournalSource.Start()
	if err != nil {
		return
	}
	return
}

// Remove existing journal ingest instance
func (manager *Manager) RemoveJrnlInstance() (err error) {
	err = manager.JournalSource.Shutdown()
	return
}
