package ingest

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender/listener"
)

// Create journal ingest instance
func (manager *InstanceManager) AddJrnlInstance(stateFile string) (err error) {
	if manager.JournalSource != nil {
		err = fmt.Errorf("cannot start a new journal instance with one running")
		return
	}

	// Create unique state file for this source
	stateFileDir := filepath.Dir(stateFile)
	stateFileName := filepath.Base(stateFile)

	newStateFileName := "journal_" + stateFileName
	newStateFile := filepath.Join(stateFileDir, newStateFileName)

	// Load last cursor
	oldPos, _ := journald.GetLastPosition(newStateFile)

	// Set the cursor position to the last known position or default to empty for the beginning
	var readPosition string
	if oldPos != "" {
		readPosition = oldPos
	} else {
		// Default to beginning if no cursor is available
		readPosition = ""
	}

	// Journal command args
	cmdArgs := []string{"--output=export", "--follow", "--no-pager"}

	if readPosition != "" {
		// Add the cursor flag to resume from the last position
		cmdArgs = append(cmdArgs, "--cursor", readPosition)
	}

	// Journal command
	cmd := exec.Command("journalctl", cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("failed to create stdout pipe for journalctl command: %v", err)
		return
	}

	// Create new context
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestCtx = context.WithValue(ingestCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	// Worker for local journal
	ingestInstance := &JrnlWorker{
		Worker:  listener.NewJrnlSource(logctx.GetTagList(manager.ctx), stdout, manager.outQueue, newStateFile),
		Command: cmd,
		cancel:  cancelInstances,
	}
	manager.JournalSource = ingestInstance

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.Worker.Namespace)
		ingestInstance.Worker.Run(ingestCtx)
	}()

	// Start command post goroutine startup
	err = manager.JournalSource.Command.Start()
	if err != nil {
		err = fmt.Errorf("failed to start journalctl command: %v", err)
		return
	}

	return
}

// Remove existing journal ingest instance
func (manager *InstanceManager) RemoveJrnlInstance() (err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	if manager.JournalSource.cancel != nil {
		manager.JournalSource.cancel()
	}
	manager.JournalSource.wg.Wait()
	if manager.JournalSource.Worker.Journal != nil {
		manager.JournalSource.Worker.Journal.Close()
	}
	return
}
