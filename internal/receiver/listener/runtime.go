package listener

import (
	"context"
	"fmt"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"strconv"
)

// Create additional ingest instance
func (manager *Manager) AddInstance() (id int, err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	id = manager.nextID
	manager.nextID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(id))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	conn, err := network.ReuseUDPPort(manager.Config.Port)
	if err != nil {
		err = fmt.Errorf("failed to reuse port: %w", err)
		return
	}

	ingestInstance := newWorker(logctx.GetTagList(manager.ctx),
		conn,
		manager.outbox,
		manager.replayCache.isReplayed)

	manager.Instances[id] = ingestInstance

	// Create new context for both watcher/assembler
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestInstance.cancel = cancelInstances
	ingestCtx = context.WithValue(ingestCtx, logctx.LoggerKey, logctx.GetLogger(manager.ctx))

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.namespace)
		ingestInstance.run(ingestCtx)
	}()
	return
}

// Remove existing instance
func (manager *Manager) RemoveInstance(id int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	ingestInstance, ok := manager.Instances[id]
	if ok {
		var dataLeft int
		if ingestInstance.conn != nil {
			// Mark draining (if supported)
			cookie, err := ebpf.GetSocketCookie(ingestInstance.conn)
			if err != nil {
				logctx.LogStdErr(manager.ctx,
					"Listener %d: failed to get cookie for socket: %w\n", id, err)
			}

			err = ebpf.MarkSocketDraining(ebpf.KernelDrainMapPath, cookie)
			if err != nil {
				logctx.LogStdErr(manager.ctx,
					"Listener %d: failed to set socket as draining: %w\n", id, err)
			}

			// Wait for drain
			dataLeft, err = network.WaitUntilEmptySocket(ingestInstance.conn)
			if err != nil {
				logctx.LogStdErr(manager.ctx,
					"Listener %d: failed to check current socket buffer size: %w\n", id, err)
			}
		}
		if ingestInstance.cancel != nil {
			ingestInstance.cancel()
		}
		if ingestInstance.conn != nil {
			// Required for listener to process cancellation when blocked
			// Theoretically... can cause deadlocks on shutdown due to close not breaking blocking read syscall
			err := ingestInstance.conn.Close()
			if err != nil {
				logctx.LogStdErr(manager.ctx,
					"Listener %d: failed to close socket: %w\n", id, err)
			}
		}

		if dataLeft > 0 {
			logctx.LogStdWarn(manager.ctx,
				"Listener %d: Socket was closed with %d bytes left in the buffer\n", id, dataLeft)
		}

		ingestInstance.wg.Wait()
		delete(manager.Instances, id)
	}
}
