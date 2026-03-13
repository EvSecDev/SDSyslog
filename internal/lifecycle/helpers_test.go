package lifecycle

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"
)

type daemonFuncAdapter struct {
	startFunc     func(ctx context.Context, key []byte) (err error)
	shutdownFunc  func()
	startFIPRFunc func() error
	stopFIPRFunc  func()
}

func (d daemonFuncAdapter) Start(ctx context.Context, key []byte) (err error) {
	if d.startFunc != nil {
		err = d.startFunc(ctx, key)
	}
	return
}

func (d daemonFuncAdapter) Shutdown() {
	if d.shutdownFunc != nil {
		d.shutdownFunc()
	}
}

func (d daemonFuncAdapter) StartFIPR() (err error) {
	if d.startFIPRFunc != nil {
		err = d.startFIPRFunc()
	}
	return
}

func (d daemonFuncAdapter) StopFIPR() {
	if d.stopFIPRFunc != nil {
		d.stopFIPRFunc()
	}
}

func (d daemonFuncAdapter) ReloadPinnedKeys() (n int, err error) {
	return
}

func setupNotifySocket(t *testing.T) (socketPath string, msgChannel <-chan string, cleanup func()) {
	t.Helper()

	dir := t.TempDir()
	socketPath = filepath.Join(dir, "sock")

	addr := net.UnixAddr{
		Name: socketPath,
		Net:  "unixgram",
	}

	conn, err := net.ListenUnixgram("unixgram", &addr)
	if err != nil {
		t.Fatalf("failed to create unixgram listener: %v", err)
	}

	messages := make(chan string, 64)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, _, err := conn.ReadFromUnix(buf)
			if err != nil {
				return
			}
			messages <- string(buf[:n])
		}
	}()

	cleanup = func() {
		err := conn.Close()
		if err != nil {
			t.Fatalf("unexpected error closing connection: %v", err)
		}
	}

	msgChannel = messages
	return
}

type failureConfig struct {
	execErr      error
	restartErr   error
	startFIPRErr error
	cmdStartErr  error
}

func checkLogForErrors(line string, errors failureConfig) (matches bool) {
	line = strings.TrimSuffix(line, "\n")
	if errors.execErr != nil {
		if strings.HasSuffix(line, errors.execErr.Error()) {
			matches = true
			return
		}
	}
	if errors.restartErr != nil {
		if strings.Contains(line, errors.restartErr.Error()) {
			matches = true
			return
		}
	}
	if errors.startFIPRErr != nil {
		if strings.HasSuffix(line, errors.startFIPRErr.Error()) {
			matches = true
			return
		}
	}
	if errors.cmdStartErr != nil {
		if strings.HasSuffix(line, errors.cmdStartErr.Error()) {
			matches = true
			return
		}
	}
	return
}
