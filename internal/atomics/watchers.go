// Helper functions that deal with atomic variables and their values
package atomics

import (
	"sync/atomic"
	"time"
)

// Waits until atomic value is 0 three consecutive times in a row, with retries and timeout
func WaitUntilZero(value *atomic.Uint64, timeout time.Duration) (reachedZero bool, lastValue uint64) {
	const successfulStreakCount = 3

	// Initial backoff duration
	backoff := 50 * time.Millisecond

	// Max backoff duration
	maxBackoff := 1 * time.Second

	deadline := time.Now().Add(timeout)
	zeroStreak := 0

	for {
		lastValue = value.Load()

		if lastValue == 0 {
			zeroStreak++
			if zeroStreak >= successfulStreakCount {
				reachedZero = true
				return
			}
		} else {
			// Reset streak if value is non-zero
			zeroStreak = 0
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			// Timed out
			reachedZero = false
			return
		}

		sleep := backoff
		if sleep > remaining {
			sleep = remaining
		}
		time.Sleep(sleep)

		// Exponential backoff with cap
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
