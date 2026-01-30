package atomics

import (
	"sync/atomic"
	"time"
)

// Tries to subtract value from the atomic source. Success if already 0.
// It retries up to maxRetries times if the CAS fails due to contention.
// Has exponential backoff, unbounded (use wisely).
func Subtract(source *atomic.Uint64, value uint64, maxRetries int) (success bool) {
	retryInterval := time.Microsecond * 10

	for i := 0; i < maxRetries; i++ {
		current := source.Load()

		if current == 0 {
			success = true
			return
		}

		var newValue uint64
		if value >= current {
			newValue = 0
		} else {
			newValue = current - value
		}

		// CAS will only succeed if the value has not changed since we last read it.
		if source.CompareAndSwap(current, newValue) {
			success = true
			return
		}

		// CAS failed due to contention, retry
		time.Sleep(retryInterval)
		retryInterval = retryInterval * 2
	}

	success = false // gave up after max attempts
	return
}
