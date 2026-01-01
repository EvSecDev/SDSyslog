package out

import (
	"context"
	"fmt"
	"net"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/output"
)

// Create and start new output instance
func (manager *InstanceManager) AddInstance(filePath string, journalEnabled bool) (err error) {
	if filePath == "" && !journalEnabled {
		err = fmt.Errorf("no outputs enabled/configured")
		return
	}

	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	instance := &OutputInstance{
		Worker: output.New(logctx.GetTagList(manager.ctx), manager.Queue),
		cancel: cancelInstance,
	}

	manager.Instance = instance

	// Add outputs
	if filePath != "" {
		var file *os.File
		file, err = os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			return
		}

		instance.Worker.FileOut = file
	}
	if journalEnabled {
		_, err = os.Stat(global.JournalSocket)
		if err != nil {
			err = fmt.Errorf("journal socket not available: %v", err)
			return
		}

		addr := &net.UnixAddr{
			Name: global.JournalSocket,
			Net:  "unixgram",
		}

		var conn *net.UnixConn
		conn, err = net.DialUnix("unixgram", nil, addr)
		if err != nil {
			err = fmt.Errorf("failed to open socket to journald: %v", err)
			return
		}
		instance.Worker.JrnlOut = conn
	}

	// Start worker
	instance.wg.Add(1)
	go func() {
		defer instance.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instance.Worker.Namespace)
		instance.Worker.Run(workerCtx)
	}()
	return
}

// Shutdown existing file output instance
func (manager *InstanceManager) RemoveInstance() {
	if manager.Instance == nil {
		return
	}
	if manager.Instance.cancel != nil {
		manager.Instance.cancel()
	}
	manager.Instance.wg.Wait()

	if manager.Instance.Worker.FileOut != nil {
		manager.Instance.Worker.FileOut.Close()
	}
	if manager.Instance.Worker.JrnlOut != nil {
		manager.Instance.Worker.JrnlOut.Close()
	}
}
