package beats

import (
	"fmt"
	"time"

	lumberjack "github.com/elastic/go-lumber/client/v2"
)

// Creates new beats (lumberjack) output module. Returns nil nil if no path.
func NewOutput(endpoint string, maxSendAttempts int) (module *OutModule, err error) {
	if endpoint == "" {
		return
	}

	module = &OutModule{
		endpoint:       endpoint,
		maxSendRetries: maxSendAttempts,
	}

	module.compression = lumberjack.CompressionLevel(0)
	module.timeout = lumberjack.Timeout(3 * time.Second)

	module.sink, err = lumberjack.SyncDial(endpoint, module.compression, module.timeout)
	if err != nil {
		err = fmt.Errorf("failed connection to beats server: %w", err)
		return
	}

	return
}

// Unsupported
func NewInput() (err error) {
	err = fmt.Errorf("beats input is currently not supported")
	return
}
