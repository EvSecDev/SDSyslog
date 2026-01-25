package lifecycle

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// Block (with timeout) until child sends readiness message over file descriptor.
func readinessReceiver(readyReader *os.File) (err error) {
	readyReader.SetReadDeadline(time.Now().Add(DefaultMaxWaitForUpdate))

	// Wait for ready message
	buf := make([]byte, len(ReadyMessage))
	_, err = io.ReadFull(readyReader, buf)
	if err != nil {
		err = fmt.Errorf("error reading readiness message: %v", err)
		return
	}

	// Verify
	msg := string(buf)
	if msg != ReadyMessage {
		err = fmt.Errorf("received message '%s', does not match expected message '%s'", msg, ReadyMessage)
		return
	}

	return
}

// Send readiness signal to file descriptor in parent-supplied environment variable file descriptor.
// Returns nil if env var does not exist.
func ReadinessSender() (err error) {
	fdStr := os.Getenv(EnvNameReadinessFD)
	if fdStr == "" {
		return // not running under updater
	}

	fd, err := strconv.Atoi(fdStr)
	if err != nil {
		err = fmt.Errorf("invalid %s: %v", EnvNameReadinessFD, err)
		return
	}

	readyPipe := os.NewFile(uintptr(fd), "ready")
	if readyPipe == nil {
		err = fmt.Errorf("failed to open %s=%d", EnvNameReadinessFD, fd)
		return
	}
	defer readyPipe.Close()

	// Send readiness message
	msg := []byte(ReadyMessage)
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
