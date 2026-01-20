package lifecycle

import (
	"fmt"
	"io"
	"os"
	"sdsyslog/internal/global"
	"strconv"
	"time"
)

// Block (with timeout) until child sends readiness message over file descriptor.
func readinessReceiver(readyReader *os.File) (err error) {
	readyReader.SetReadDeadline(time.Now().Add(global.DefaultMaxWaitForUpdate))

	// Wait for ready message
	buf := make([]byte, len(global.ReadyMessage))
	_, err = io.ReadFull(readyReader, buf)
	if err != nil {
		err = fmt.Errorf("error reading readiness message: %v", err)
		return
	}

	// Verify
	msg := string(buf)
	if msg != global.ReadyMessage {
		err = fmt.Errorf("received message '%s', does not match expected message '%s'", msg, global.ReadyMessage)
		return
	}

	return
}

// Send readiness signal to file descriptor in parent-supplied environment variable file descriptor.
// Returns nil if env var does not exist.
func ReadinessSender() (err error) {
	fdStr := os.Getenv(global.EnvNameReadinessFD)
	if fdStr == "" {
		return // not running under updater
	}

	fd, err := strconv.Atoi(fdStr)
	if err != nil {
		err = fmt.Errorf("invalid %s: %v", global.EnvNameReadinessFD, err)
		return
	}

	readyPipe := os.NewFile(uintptr(fd), "ready")
	if readyPipe == nil {
		err = fmt.Errorf("failed to open %s=%d", global.EnvNameReadinessFD, fd)
		return
	}
	defer readyPipe.Close()

	// Send readiness message
	msg := []byte(global.ReadyMessage)
	for len(msg) > 0 {
		var bytesWritten int
		bytesWritten, err = readyPipe.Write(msg)
		if err != nil {
			err = fmt.Errorf("failed to send readiness message: %v", err)
			return
		}
		msg = msg[bytesWritten:]
	}

	return
}

// Uses parent-supplied environment variable file descriptor to determine when parent exits.
// Returns nil if env var does not exist.
func WaitForParentExit() (err error) {
	fdStr := os.Getenv(global.EnvNameAlivenessFD)
	if fdStr == "" {
		return // Not under updater
	}

	fd, err := strconv.Atoi(fdStr)
	if err != nil {
		err = fmt.Errorf("invalid %s: %v", global.EnvNameAlivenessFD, err)
		return
	}

	f := os.NewFile(uintptr(fd), "parent-alive")
	if f == nil {
		err = fmt.Errorf("failed to open %s=%d", global.EnvNameAlivenessFD, fd)
		return
	}
	defer f.Close()

	buf := make([]byte, 1)
	_, err = f.Read(buf)
	if err == io.EOF {
		// Parent exited
		err = nil
		return
	} else if err != nil {
		err = fmt.Errorf("error waiting for parent exit: %v", err)
		return
	}
	return
}
