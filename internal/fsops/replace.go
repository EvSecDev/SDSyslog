// Package containing complex file system operations
package fsops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func AtomicFileReplace(src, dst string, perm os.FileMode) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		err = fmt.Errorf("open src: %w", err)
		return
	}
	defer func() {
		_ = srcFile.Close()
	}()

	// Unique name for right now and this process
	tempPath := fmt.Sprintf("%s.tmp.%d.%d", dst, os.Getpid(), time.Now().UnixNano())

	tmpFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		err = fmt.Errorf("create temp file: %w", err)
		return
	}

	// ensure cleanup on failure
	defer func() {
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	_, err = io.Copy(tmpFile, srcFile)
	if err != nil {
		err = fmt.Errorf("copy: %w", err)
		return
	}

	err = tmpFile.Sync()
	if err != nil {
		err = fmt.Errorf("sync temp file: %w", err)
		return
	}

	err = tmpFile.Close()
	if err != nil {
		err = fmt.Errorf("close temp file: %w", err)
		return
	}

	dir, err := os.Open(filepath.Dir(dst))
	if err != nil {
		err = fmt.Errorf("open dir: %w", err)
		return
	}
	defer func() {
		_ = dir.Close()
	}()

	err = dir.Sync()
	if err != nil {
		err = fmt.Errorf("sync dir: %w", err)
		return
	}

	err = os.Rename(tempPath, dst)
	if err != nil {
		err = fmt.Errorf("rename commit: %w", err)
		return
	}

	return
}
