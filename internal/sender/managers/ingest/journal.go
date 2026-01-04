package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sdsyslog/internal/externalio/journald"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender/listener"
	"strings"
	"time"
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
		cmdArgs = append(cmdArgs, "--after-cursor", readPosition)
	}

	// Journal command
	cmd := exec.Command("journalctl", cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("failed to create stdout pipe for journalctl command: %v", err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		err = fmt.Errorf("failed to create stderr pipe for journalctl command: %v", err)
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

	// Assert to file to set a deadline
	errFile := stderr.(*os.File)
	defer errFile.Close()
	err = errFile.SetReadDeadline(time.Now().Add(25 * time.Millisecond))
	if err != nil {
		err = fmt.Errorf("failed to set deadline on stderr reader: %v", err)
		return
	}

	buf := make([]byte, 4096)
	bytesRead, err := errFile.Read(buf)
	if err != nil {
		if !os.IsTimeout(err) {
			err = fmt.Errorf("failed to read journalctl stderr: %v", err)
			return
		}
		err = nil
	}

	// Check stderr for potential stop errors, treat any stderr as fatal
	journalError := strings.ToLower(string(buf[:bytesRead]))
	if journalError != "" {
		err = fmt.Errorf("found fatal error after journald watch startup: %s", journalError)
		return
	}

	// Remove deadline for future blocking reads (just in case)
	errFile.SetReadDeadline(time.Time{})
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
