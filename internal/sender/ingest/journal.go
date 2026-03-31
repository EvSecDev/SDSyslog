package ingest

import (
	"fmt"
	"sdsyslog/internal/iomodules/journald"
)

// Create journal ingest instance
func (manager *Manager) AddJrnlInstance(stateFile string) (err error) {
	if manager.JournalSource != nil {
		err = fmt.Errorf("cannot start a new journal instance with one running")
		return
	}

	filters := manager.Config.SourceDropFilters[JrnlSource]
	manager.JournalSource, err = journald.NewInput(manager.ctx, stateFile, filters, manager.outQueue)
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
