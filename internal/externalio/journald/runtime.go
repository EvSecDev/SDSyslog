package journald

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Starts journalctl command. Use after reader startup
func (mod *InModule) Start() (err error) {
	// Start command post goroutine startup
	err = mod.cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start journalctl command: %v", err)
		return
	}
	return
}

// Checks running command for any fatal errors.
func (mod *InModule) CheckError() (err error) {
	// Assert to file to set a deadline
	errFile := mod.err.(*os.File)
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

// Gracefully stops module
func (mod *InModule) Shutdown() (err error) {
	if mod == nil {
		return
	}
	if mod.sink != nil {
		err = mod.sink.Close()
	}
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
