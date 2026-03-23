package listener

import (
	"fmt"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"strconv"
	"time"
)

// Create additional ingest instance
func (manager *Manager) AddInstance() (id int, err error) {
	if manager == nil {
		return
	}

	conn, err := network.ReuseUDPPort(manager.Config.Port)
	if err != nil {
		err = fmt.Errorf("failed to reuse port: %w", err)
		return
	}

	ingestInstance := manager.newWorker(conn)

	for {
		oldListPtr := manager.Instances.Load()
		oldList := *oldListPtr

		// Copy slice
		newList := make([]*Instance, len(oldList)+1)
		copy(newList, oldList)
		newList[len(oldList)] = ingestInstance

		if manager.Instances.CompareAndSwap(oldListPtr, &newList) {
			id = len(oldList)
			break
		}
	}

	// Create new context for worker
	ingestInstance.ctx, ingestInstance.cancel = logctx.NewCancelWithValues(manager.ctx, logctx.NSListen, strconv.Itoa(id))

	ingestInstance.wg.Add(1)
	go func() {
		defer ingestInstance.wg.Done()
		ingestInstance.run()
	}()
	return
}

// Removes the last added instance
func (manager *Manager) RemoveLastInstance() (removedID int) {
	if manager == nil {
		return
	}

	var ingestInstance *Instance
	for {
		oldListPtr := manager.Instances.Load()
		oldList := *oldListPtr

		if len(oldList) == 0 {
			return
		}

		lastIndex := len(oldList) - 1
		ingestInstance = oldList[lastIndex]

		newList := make([]*Instance, lastIndex)
		copy(newList, oldList[:lastIndex])

		if manager.Instances.CompareAndSwap(oldListPtr, &newList) {
			removedID = lastIndex
			break
		}
	}
	if ingestInstance == nil {
		return
	}

	var dataLeft int
	if ingestInstance.conn != nil {
		// Mark draining (if supported)
		cookie, err := ebpf.GetSocketCookie(ingestInstance.conn)
		if err != nil {
			logctx.LogStdErr(manager.ctx,
				"Listener %d: failed to get cookie for socket: %w\n", removedID, err)
		}

		err = ebpf.MarkSocketDraining(ebpf.KernelDrainMapPath, cookie)
		if err != nil {
			logctx.LogStdErr(manager.ctx,
				"Listener %d: failed to set socket as draining: %w\n", removedID, err)
		}

		// Wait for drain
		dataLeft, err = network.WaitUntilEmptySocket(ingestInstance.conn)
		if err != nil {
			logctx.LogStdErr(manager.ctx,
				"Listener %d: failed to check current socket buffer size: %w\n", removedID, err)
		}
	}
	if ingestInstance.cancel != nil {
		ingestInstance.cancel()
	}
	if ingestInstance.conn != nil {
		// Required for listener to process cancellation when blocked

		// Closing with timeout
		// Small chance that it causes deadlocks on shutdown due to close not breaking blocking read syscall
		closeWaitTime := 2 * time.Second
		done := make(chan struct{})
		go func() {
			err := ingestInstance.conn.Close()
			if err != nil {
				logctx.LogStdErr(manager.ctx,
					"Listener %d: failed to close socket: %w\n", removedID, err)
			}
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(closeWaitTime):
			// Leaving as info print as it's neither warning nor error
			logctx.LogStdInfo(manager.ctx, "Timeout: listener socket did not close within %v seconds after cancellation (no error)\n",
				closeWaitTime.Seconds())
			return
		}
	}

	if dataLeft > 0 {
		logctx.LogStdWarn(manager.ctx,
			"Listener %d: Socket was closed with %d bytes left in the buffer\n", removedID, dataLeft)
	}

	ingestInstance.wg.Wait()
	return
}
