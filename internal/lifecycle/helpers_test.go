package lifecycle

import (
	"context"
	"net"
	"path/filepath"
	"sdsyslog/internal/tests/utils"
	"testing"
)

type daemonFuncAdapter struct {
	initFunc      func(key []byte) (err error)
	startFunc     func() (err error)
	shutdownFunc  func()
	startFIPRFunc func() error
	stopFIPRFunc  func()
}

func (daemonAdapter daemonFuncAdapter) Init(key []byte) (err error) {
	if daemonAdapter.initFunc != nil {
		err = daemonAdapter.initFunc(key)
	}
	return
}

func (daemonAdapter daemonFuncAdapter) Start() (err error) {
	if daemonAdapter.startFunc != nil {
		err = daemonAdapter.startFunc()
	}
	return
}

func (daemonAdapter daemonFuncAdapter) Shutdown() {
	if daemonAdapter.shutdownFunc != nil {
		daemonAdapter.shutdownFunc()
	}
}

func (daemonAdapter daemonFuncAdapter) StartFIPR() (err error) {
	if daemonAdapter.startFIPRFunc != nil {
		err = daemonAdapter.startFIPRFunc()
	}
	return
}

func (daemonAdapter daemonFuncAdapter) StopFIPR() {
	if daemonAdapter.stopFIPRFunc != nil {
		daemonAdapter.stopFIPRFunc()
	}
}

func (daemonAdapter daemonFuncAdapter) ReloadSigningKeys() (n int, err error) {
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
			bytesRead, _, err := conn.ReadFromUnix(buf)
			if err != nil {
				return
			}
			messages <- string(buf[:bytesRead])
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
