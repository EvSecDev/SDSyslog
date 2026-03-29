package file

import (
	"io"
	"os"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

type OutModule struct {
	sink        io.WriteCloser
	batchSize   int
	batchBuffer *[]string
}

type InModule struct {
	Namespace     []string
	localHostname string

	// Read Source
	sink     *os.File
	filePath string
	filters  []protocol.MessageFilter

	watcher xWatcher

	// State
	stateFile         string
	currentReadOffset int64
	currentReadID     fileID

	outbox  *mpmc.Queue[protocol.Message]
	metrics MetricStorage
}

// Cross-platform file watching worker
type xWatcher interface {
	Start()
	Stop()
	FileChanged() <-chan struct{}
	FileRotated() <-chan struct{}
}

type fileID struct {
	dev uint64
	ino uint64
}
