package fsops

import (
	"fmt"
	"path/filepath"
)

func NormalizePath(path string) (realPath string, err error) {
	realPath, err = filepath.EvalSymlinks(path)
	if err != nil {
		err = fmt.Errorf("failed to resolve symbolic links path: %w", err)
		return
	}
	realPath, err = filepath.Abs(path)
	if err != nil {
		err = fmt.Errorf("failed to get absolute path for path: %w", err)
		return
	}
	return
}
