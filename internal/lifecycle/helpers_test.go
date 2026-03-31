package lifecycle

import (
	"context"
	"net"
	"path/filepath"
	"sdsyslog/internal/tests/utils"
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

func (d daemonFuncAdapter) ReloadSigningKeys() (n int, err error) {
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

func checkLogForErrors(t *testing.T, ctx context.Context, errors failureConfig) {
	t.Helper()

	if errors.execErr != nil {
		// Gather any logs from ctx logger
		_, lerr := utils.MatchLogCtxErrors(ctx, errors.execErr.Error(), nil)
		if lerr != nil {
			t.Errorf("%v", lerr)
		}
	}
	if errors.restartErr != nil {
		_, lerr := utils.MatchLogCtxErrors(ctx, errors.restartErr.Error(), nil)
		if lerr != nil {
			t.Errorf("%v", lerr)
		}
	}
	if errors.startFIPRErr != nil {
		_, lerr := utils.MatchLogCtxErrors(ctx, errors.startFIPRErr.Error(), nil)
		if lerr != nil {
			t.Errorf("%v", lerr)
		}
	}
	if errors.cmdStartErr != nil {
		_, lerr := utils.MatchLogCtxErrors(ctx, errors.cmdStartErr.Error(), nil)
		if lerr != nil {
			t.Errorf("%v", lerr)
		}
	}
}
