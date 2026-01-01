// Helper functions that deal with atomic variables and their values
package atomics

import (
	"sync/atomic"
	"time"
)

// Waits until atomic value is 0 three consecutive times in a row, with retries and timeout
func WaitUntilZero(value *atomic.Uint64) (reachedZero bool, lastValue uint64) {
	const successfulStreakCount int = 3

	// Initial backoff duration
	backoffDuration := 50 * time.Millisecond

	// Max backoff duration
	maxBackoff := 1 * time.Second

	// Maximum number of iterations
	maxIterations := 30

	// Track consecutive values at 0
	zeroStreak := 0

	// Retry loop with exponential backoff
	for i := 0; i < maxIterations; i++ {
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

		// Sleep for the backoff duration
		time.Sleep(backoffDuration)

		// Increase the backoff duration exponentially
		if backoffDuration < maxBackoff {
			backoffDuration *= 2
			if backoffDuration > maxBackoff {
				backoffDuration = maxBackoff
			}
		}
	}

	// Timed out
	return
}
