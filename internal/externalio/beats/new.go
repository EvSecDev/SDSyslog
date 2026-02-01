package beats

import (
	"fmt"
	"time"

	lumberjack "github.com/elastic/go-lumber/client/v2"
)

// Creates new beats (lumberjack) output module. Returns nil nil if no path.
func NewOutput(endpoint string) (module *OutModule, err error) {
	if endpoint == "" {
		return
	}

	compression := lumberjack.CompressionLevel(0)
	timeout := lumberjack.Timeout(3 * time.Second)

	ljClient, err := lumberjack.SyncDial(endpoint, compression, timeout)
	if err != nil {
		err = fmt.Errorf("failed connection to beats server: %w", err)
		return
	}

	module = &OutModule{
		sink: ljClient,
	}
	return
}
