package ingest

import (
	"fmt"
	"io"
	"sdsyslog/internal/iomodules/generic"
)

// Create journal ingest instance
func (manager *Manager) AddRawInstance(sourceReader io.ReadCloser) (err error) {
	if manager.RawSource != nil {
		err = fmt.Errorf("cannot start a new raw instance with one running")
		return
	}

	manager.RawSource, err = generic.NewInput(manager.ctx, sourceReader, manager.outQueue)
	if err != nil {
		return
	}

	err = manager.RawSource.Start()
	if err != nil {
		return
	}
	return
}

// Remove existing journal ingest instance
func (manager *Manager) RemoveRawInstance() (err error) {
	err = manager.RawSource.Shutdown()
	return
}
