package fiprrecv

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/fipr"
	"strconv"
	"time"
)

// Creates new fragment inter-process routing receiver instance
func New(ctx context.Context, socketDirectoryPath string, localRouterView shard.RoutingView) (instance *Instance) {
	fileName := fipr.SocketFileNamePrefix + strconv.Itoa(os.Getpid()) + fipr.SocketFileNameSuffix
	path := filepath.Join(socketDirectoryPath, fileName)

	ctx = logctx.AppendCtxTag(ctx, logctx.NSmFIPR)

	instance = &Instance{
		Namespace:        logctx.GetTagList(ctx),
		socketPath:       path,
		hmacSecret:       wrappers.GetSharedSecret(),
		localRoutingView: localRouterView,
		Metrics:          MetricStorage{},
		ctx:              ctx,
	}
	return
}

// Starts go routine for receiving inter-process fragments via unix socket
func (instance *Instance) Start() (err error) {
	// Remove existing socket file (enforce single listener at a time)
	_, err = os.Stat(instance.socketPath)
	if err == nil {
		err = os.Remove(instance.socketPath)
		if err != nil {
			err = fmt.Errorf("failed to remove existing socket path: %w", err)
			return
		}
	}

	// Create IPC directory if missing
	socketDir := filepath.Dir(instance.socketPath)
	_, err = os.Stat(socketDir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(socketDir, 0700)
		if err != nil {
			err = fmt.Errorf("failed to create missing socket parent directory: %w", err)
			return
		}
	} else if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to check existence of socket directory: %w", err)
		return
	}

	// Create Unix socket listener
	instance.listener, err = net.Listen("unix", instance.socketPath)
	if err != nil {
		err = fmt.Errorf("failed creating Unix socket listener: %w", err)
		return
	}

	workerCtx, cancel := context.WithCancel(context.Background())
	instance.cancel = cancel
	workerCtx = context.WithValue(workerCtx, logctx.LoggerKey, logctx.GetLogger(instance.ctx))

	instance.wg.Add(1)
	go func() {
		defer instance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(workerCtx, logctx.GetTagList(instance.ctx))
		instance.Run(ingestCtx)
	}()

	return
}

// Stops fragment receiver listener.
// Attempts to wait for no more connections, otherwise force close within 4 seconds.
func (instance *Instance) Stop() {
	doneSignal := make(chan struct{})
	go func() {
		defer close(doneSignal)
		instance.wgConn.Wait()
	}()
	select {
	case <-doneSignal:
		// Graceful shutdown
	case <-time.After(4 * time.Second):
		// Timed out, close anyway
	}
	if instance.cancel != nil {
		instance.cancel()
	}
	if instance.listener != nil {
		err := instance.listener.Close()
		if err != nil {
			logctx.LogStdErr(instance.ctx,
				"failed to close socket listener: %w\n", err)
		}
	}

	// Cleanup socket file
	err := os.Remove(instance.socketPath)
	if err != nil && !os.IsNotExist(err) {
		logctx.LogStdWarn(instance.ctx,
			"Fragment Inter-Process Router socket file %s: removal failed, this could cause slow downs in future packet processing: %w\n",
			instance.socketPath, err)
	}

	instance.wg.Wait()
}
