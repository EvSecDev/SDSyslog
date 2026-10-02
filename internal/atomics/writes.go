package atomics

import (
	"sync/atomic"
	"time"
)

// Tries to subtract value from the atomic source. Success if already 0.
// It retries up to 128 times if the CAS fails due to contention.
// Has exponential backoff, bounded to 1 millisecond per retry.
func Subtract(source *atomic.Uint64, value uint64) (success bool) {
	const maxSleep time.Duration = 1 * time.Millisecond
	retryInterval := time.Microsecond * 10

	// Contention is transient.
	const maxRetries = 128

	for range maxRetries {
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
		if retryInterval > maxSleep {
			retryInterval = maxSleep
		}
		time.Sleep(retryInterval)
		retryInterval *= 2
	}

	success = false // gave up after max attempts
	return
}
