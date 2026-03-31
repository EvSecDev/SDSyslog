package file

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules/file/inotify"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new file input module. Returns nil nil if no path.
func NewInput(ctx context.Context, filePath string, baseStateFile string, filters []protocol.MessageFilter, queue *mpmc.Queue[protocol.Message]) (module *InModule, err error) {
	if filePath == "" {
		return
	}

	for index, filter := range filters {
		err = filter.Validate()
		if err != nil {
			err = fmt.Errorf("invalid message filter at index %d: %w", index, err)
			return
		}
	}

	// Create unique state file for this source
	stateFileDir := filepath.Dir(baseStateFile)
	stateFileName := filepath.Base(baseStateFile)

	newStateFileName := base64.RawURLEncoding.EncodeToString([]byte(filePath)) + "_" + stateFileName // Using full file path as prefix to state file
	newStateFile := filepath.Join(stateFileDir, newStateFileName)

	// New context for file
	newNamespace := append(logctx.GetTagList(ctx), logctx.NSoFile, filepath.Base(filePath))
	modCtx := logctx.OverwriteCtxTag(ctx, newNamespace)
	modCtx, cancel := context.WithCancel(modCtx)

	module = &InModule{
		filePath:  filePath,
		stateFile: newStateFile,
		filters:   filters,
		outbox:    queue,
		metrics:   MetricStorage{},

		ctx:    modCtx,
		cancel: cancel,
	}

	module.sink, err = os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		err = fmt.Errorf("failed to open source file: %w", err)
		return
	}

	// Seek to beginning or last position
	module.currentReadID.ino, module.currentReadOffset, err = getLastPosition(module.filePath, module.stateFile)
	if err != nil {
		logctx.LogStdWarn(ctx, "failed to get position of last source file read for '%s': %w\n", module.filePath, err)
		// No error, resume from beginning
	}
	_, err = module.sink.Seek(module.currentReadOffset, io.SeekStart)
	if err != nil {
		err = fmt.Errorf("failed to resume last source file read position for '%s': %w", module.filePath, err)
		return
	}

	module.localHostname, err = os.Hostname()
	if err != nil {
		err = fmt.Errorf("failed to retrieve local hostname: %w", err)
		return
	}

	// Get watcher for OS (also gates file inode cross platform problem)
	switch runtime.GOOS {
	case global.GOOSLinux:
		// Linux inotify event watcher
		module.watcher, err = inotify.New(ctx, module.filePath)
		if err != nil {
			err = fmt.Errorf("failed to create new watcher for file source %q: %w", module.filePath, err)
			return
		}
	default:
		// I'm not supporting windows, get a better os
		err = fmt.Errorf("file ingest is not currently supported on OS %q", runtime.GOOS)
		return
	}

	return
}

// Creates new file output module. Returns nil nil if no path.
func NewOutput(filePath string, batchSize int) (module *OutModule, err error) {
	if filePath == "" {
		return
	}

	const defaultBatchSize int = 20
	if batchSize == 0 {
		batchSize = defaultBatchSize
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return
	}

	module = &OutModule{
		sink:        file,
		batchSize:   batchSize,
		batchBuffer: &[]string{},
	}
	return
}
