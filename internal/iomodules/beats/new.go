package beats

import (
	"fmt"
	"time"

	lumberjack "github.com/elastic/go-lumber/client/v2"
)

// Creates new beats (lumberjack) output module. Returns nil, nil if no path.
// Will wait until dial to beats server succeeds. Maximum time is defined as startupRetryDuration + 5 seconds (sleep+dial timeout)
func NewOutput(endpoint string, maxSendAttempts int, startupRetryDuration time.Duration) (module *OutModule, err error) {
	if endpoint == "" {
		return
	}

	if startupRetryDuration < minStartupDuration {
		err = fmt.Errorf("minimum retry startup duration must be at least %.0f seconds", minStartupDuration.Seconds())
		return
	}

	module = &OutModule{
		endpoint:       endpoint,
		maxSendRetries: maxSendAttempts,
	}

	module.compression = lumberjack.CompressionLevel(0)
	module.timeout = lumberjack.Timeout(dialTimeout)

	startupStartTime := time.Now()
	startupEndTime := startupStartTime.Add(startupRetryDuration)
	retryCount := 0
	for {
		module.sink, err = lumberjack.SyncDial(endpoint, module.compression, module.timeout)
		if err != nil {
			if time.Now().After(startupEndTime) {
				// Dial did not succeed in time
				err = fmt.Errorf("failed connection to beats server after %d retries: %w", retryCount, err)
				return
			} else {
				// Within retry period, wait and retry
				time.Sleep(startupRetryDelay)
				retryCount++
				continue
			}
		}
		break
	}

	return
}

// Unsupported
func NewInput() (err error) {
	err = fmt.Errorf("beats input is currently not supported")
	return
}
