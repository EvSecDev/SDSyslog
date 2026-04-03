// Basic text parsing functions not tied to a specific internal use case
package parsing

import (
	"fmt"
	"strings"
	"time"
)

// Trims excessive decimal places in a time duration string formatted output to desired numDecimals places.
// Only trims, does NOT round up or down.
func TrimDurationPrecision(duration time.Duration, numDecimals int) (formatted string) {
	formatted = duration.String()

	dot := strings.IndexByte(formatted, '.')
	if dot == -1 {
		return
	}

	// find where the numeric fraction ends
	i := dot + 1
	for i < len(formatted) && formatted[i] >= '0' && formatted[i] <= '9' {
		i++
	}

	if numDecimals == 0 {
		// remove fractional part completely
		formatted = formatted[:dot] + formatted[i:]
		return
	}

	fraction := formatted[dot+1 : i]
	if len(fraction) <= numDecimals {
		return
	}

	formatted = formatted[:dot+1+numDecimals] + formatted[i:]
	return
}

// Checks to see if time duration is a clean divisor of allowed parent duration units.
func VerifyWholeDuration(interval time.Duration) (err error) {
	if interval <= 0 {
		err = fmt.Errorf("interval must be positive")
		return
	}

	// Allowed parents
	parents := []time.Duration{
		time.Millisecond,
		time.Second,
		time.Minute,
	}

	for _, parent := range parents {
		if parent%interval == 0 {
			return
		}
	}

	err = fmt.Errorf(
		"interval %v must divide evenly into 1ms, 1s, or 1m",
		interval,
	)
	return
}
