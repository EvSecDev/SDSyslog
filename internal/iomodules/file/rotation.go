package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sdsyslog/internal/logctx"
	"time"
)

// Opens log file for module to get new inode
func (mod *InModule) reopenLogfile(ctx context.Context) (err error) {
	// Retrieve new file inode
	maxRetries := 5
	delay := 100 * time.Millisecond
	maxDelay := time.Minute

	var fileInfo os.FileInfo
	for range maxRetries {
		fileInfo, err = os.Stat(mod.filePath)
		if err == nil {
			break
		}

		// Non-retryable error
		if errors.Is(err, os.ErrPermission) {
			err = fmt.Errorf("unable to stat new source file: %w", err)
			return
		}

		// Wait and increment backoff
		time.Sleep(delay)

		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}
	if err != nil {
		err = fmt.Errorf("failed to stat new rotated log file after %d retires within %.0f seconds: %w",
			maxRetries, maxDelay.Seconds(), err)
		return
	}

	var newFileID fileID
	newFileID, err = getFileID(fileInfo)
	if err != nil {
		err = fmt.Errorf("failed to get file ID: %w", err)
		return
	}

	// Not actual rotation
	if mod.currentReadID.ino == newFileID.ino {
		return
	}

	// Save new file inode to state var
	mod.currentReadID.ino = newFileID.ino

	// Reopen at file path
	err = mod.sink.Close()
	if err != nil {
		logctx.LogStdWarn(ctx, "failed to close previous file '%s': %w\n", mod.filePath, err)
	}
	mod.sink, err = os.Open(mod.filePath)
	if err != nil {
		err = fmt.Errorf("failed to reopen rotated log file: %w", err)
		return
	}

	// Reset offset position for new file
	mod.currentReadOffset = 0
	return
}
