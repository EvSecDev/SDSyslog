package in

import (
	"context"
	"fmt"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/receiver/listener"
	"strconv"
)

// Create additional ingest instance
func (manager *InstanceManager) AddInstance() (id int, err error) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	id = manager.nextID
	manager.nextID++

	// Add log context
	manager.ctx = logctx.AppendCtxTag(manager.ctx, strconv.Itoa(id))
	defer func() { manager.ctx = logctx.RemoveLastCtxTag(manager.ctx) }()

	conn, err := network.ReuseUDPPort(manager.port)
	if err != nil {
		err = fmt.Errorf("failed to reuse port: %v", err)
		return
	}

	ingestInstance := &Instance{
		conn:     conn,
		Listener: listener.New(logctx.GetTagList(manager.ctx), conn, manager.outbox),
	}

	manager.Instances[id] = ingestInstance

	// Create new context for both watcher/assembler
	ingestCtx, cancelInstances := context.WithCancel(context.Background())
	ingestInstance.cancel = cancelInstances
	ingestCtx = context.WithValue(ingestCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestCtx := logctx.OverwriteCtxTag(ingestCtx, ingestInstance.Listener.Namespace)
		ingestInstance.Listener.Run(ingestCtx)
	}()
	return
}

// Remove existing instance
func (manager *InstanceManager) RemoveInstance(id int) {
	manager.Mu.Lock()
	defer manager.Mu.Unlock()

	ingestInstance, ok := manager.Instances[id]
	if ok {
		if ingestInstance.cancel != nil {
			ingestInstance.cancel()
		}
		if ingestInstance.conn != nil {
			// Mark draining (if supported)
			cookie, err := ebpf.GetSocketCookie(ingestInstance.conn)
			if err != nil {
				logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.ErrorLog,
					"Listener %d: failed to get cookie for socket: %v\n", id, err)
			}

			err = ebpf.MarkSocketDraining(global.KernelDrainMapPath, cookie)
			if err != nil {
				logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.ErrorLog,
					"Listener %d: failed to set socket as draining: %v\n", id, err)
			}

			// Wait for drain
			dataLeft, err := network.WaitUntilEmptySocket(ingestInstance.conn)
			if err != nil {
				logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.ErrorLog,
					"Listener %d: failed to check current socket buffer size: %v\n", id, err)
			}
			if dataLeft > 0 {
				logctx.LogEvent(manager.ctx, global.VerbosityStandard, global.WarnLog,
					"Listener %d: Socket is being closed with %d bytes left in the buffer\n", id, dataLeft)
			}

			ingestInstance.conn.Close() // Required for listener to process cancellation
		}

		ingestInstance.wg.Wait()
		delete(manager.Instances, id)
	}
}
