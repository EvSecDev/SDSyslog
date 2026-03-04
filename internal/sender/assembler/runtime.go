package assembler

import (
	"sdsyslog/internal/logctx"
	"strconv"
)

// Create new packaging instance
func (manager *Manager) AddInstance() (id int) {
	if manager == nil {
		return
	}

	// Create new worker instance
	newWorker := manager.newWorker()

	for {
		oldListPtr := manager.Instances.Load()
		oldList := *oldListPtr

		// Copy slice
		newList := make([]*Instance, len(oldList)+1)
		copy(newList, oldList)
		newList[len(oldList)] = newWorker

		if manager.Instances.CompareAndSwap(oldListPtr, &newList) {
			id = len(oldList)
			break
		}
	}

	// Create new context for worker
	newWorker.ctx, newWorker.cancel = logctx.NewCancelWithValues(manager.ctx, strconv.Itoa(id), logctx.NSAssm)

	newWorker.wg.Add(1)
	go func() {
		// Run the assembler
		defer newWorker.wg.Done()
		newWorker.run()
	}()
	return
}

// Remove existing packaging instance
func (manager *Manager) RemoveLastInstance() (removedID int) {
	if manager == nil {
		return
	}

	var worker *Instance
	for {
		oldListPtr := manager.Instances.Load()
		oldList := *oldListPtr

		if len(oldList) == 0 {
			return
		}

		lastIndex := len(oldList) - 1
		worker = oldList[lastIndex]

		newList := make([]*Instance, lastIndex)
		copy(newList, oldList[:lastIndex])

		if manager.Instances.CompareAndSwap(oldListPtr, &newList) {
			removedID = lastIndex
			break
		}
	}
	if worker == nil {
		return
	}

	if worker.cancel != nil {
		worker.cancel()
	}
	worker.wg.Wait()
	return
}
