package processor

import (
	"sdsyslog/internal/logctx"
	"strconv"
)

// Create additional ingest instance
func (manager *Manager) AddInstance() (id int) {
	if manager == nil {
		return
	}

	processor := manager.newWorker()

	for {
		oldListPtr := manager.Instances.Load()
		oldList := *oldListPtr

		// Copy slice
		newList := make([]*Instance, len(oldList)+1)
		copy(newList, oldList)
		newList[len(oldList)] = processor

		if manager.Instances.CompareAndSwap(oldListPtr, &newList) {
			id = len(oldList)
			break
		}
	}

	// Create new context for worker
	processor.ctx, processor.cancel = logctx.NewCancelWithValues(manager.ctx, strconv.Itoa(id), logctx.NSProc)

	processor.wg.Add(1)
	go func() {
		defer processor.wg.Done()
		processor.run()
	}()
	return
}

// Remove existing instance
func (manager *Manager) RemoveLastInstance() (removedID int) {
	if manager == nil {
		return
	}

	var processor *Instance
	for {
		oldListPtr := manager.Instances.Load()
		oldList := *oldListPtr

		if len(oldList) == 0 {
			return
		}

		lastIndex := len(oldList) - 1
		processor = oldList[lastIndex]

		newList := make([]*Instance, lastIndex)
		copy(newList, oldList[:lastIndex])

		if manager.Instances.CompareAndSwap(oldListPtr, &newList) {
			removedID = lastIndex
			break
		}
	}
	if processor == nil {
		return
	}

	if processor.cancel != nil {
		processor.cancel()
	}

	processor.wg.Wait()
	return
}
