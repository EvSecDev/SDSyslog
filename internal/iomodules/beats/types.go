package beats

import (
	lumberjack "github.com/elastic/go-lumber/client/v2"
)

type OutModule struct {
	sink *lumberjack.SyncClient
}
