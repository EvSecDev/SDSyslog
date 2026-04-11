package journald

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Starts journalctl command. Use after reader startup
func (mod *InModule) Start() (err error) {
	// Start reader in go routine
	mod.wg.Add(1)
	go mod.reader()

	// Start command post goroutine startup
	err = mod.cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start journalctl command: %w", err)
		return
	}

	// Wait until reader is started
	<-mod.readerReady

	// Assert to file to set a deadline
	errFile, ok := mod.err.(*os.File)
	if !ok {
		err = fmt.Errorf("failed to type assert stderr from journalctl command to os.File")
		return
	}
	defer func() {
		lerr := errFile.Close()
		if lerr != nil && err == nil {
			err = fmt.Errorf("failed to close stderr file: %w", lerr)
		}
	}()
	err = errFile.SetReadDeadline(time.Now().Add(25 * time.Millisecond))
	if err != nil {
		err = fmt.Errorf("failed to set deadline on stderr reader: %w", err)
		return
	}

	buf := make([]byte, 4096)
	bytesRead, err := errFile.Read(buf)
	if err != nil {
		if !os.IsTimeout(err) {
			err = fmt.Errorf("failed to read journalctl stderr: %w", err)
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
	err = errFile.SetReadDeadline(time.Time{})
	if err != nil {
		err = fmt.Errorf("failed to clear read deadline on stderr reader: %w", err)
		return
	}
	return
}

// Gracefully stops module
func (mod *InModule) Shutdown() (err error) {
	if mod == nil {
		return
	}

	if mod.cancel != nil {
		mod.cancel()
	}

	if mod.sink != nil {
		err = mod.sink.Close()
	}

	mod.wg.Wait()
	return
}

// Gracefully stops module (err always nil)
func (mod *OutModule) Shutdown() (err error) {
	if mod == nil {
		return
	}
	if mod.sink != nil {
		mod.sink.CloseIdleConnections()
	}
	return
}
