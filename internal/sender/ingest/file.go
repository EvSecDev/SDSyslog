package ingest

import (
	"fmt"
	"path/filepath"
	"sdsyslog/internal/iomodules/file"
)

// Create file ingest instance
func (manager *Manager) AddFileInstance(filePath string, stateFile string) (err error) {
	manager.FileSourceMu.Lock()
	defer manager.FileSourceMu.Unlock()

	filename := filepath.Base(filePath)
	_, ok := manager.FileSources[filename]
	if ok {
		err = fmt.Errorf("cannot start a new file instance with one running for path '%s'", filePath)
		return
	}

	// Worker for this file
	filters := manager.Config.SourceDropFilters[FileSource]
	new, err := file.NewInput(manager.ctx, filePath, stateFile, filters, manager.outQueue)
	if err != nil {
		return
	}

	err = new.Start()
	if err != nil {
		return
	}

	manager.FileSources[filePath] = new
	return
}

// Remove existing file ingest instance
func (manager *Manager) RemoveFileInstance(filename string) (err error) {
	manager.FileSourceMu.Lock()
	defer manager.FileSourceMu.Unlock()

	fileSource, ok := manager.FileSources[filename]
	if !ok {
		err = fmt.Errorf("no file source for '%s'", filename)
		return
	}

	err = fileSource.Shutdown()
	if err != nil {
		return
	}
	manager.FileSources[filename] = nil
	return
}
