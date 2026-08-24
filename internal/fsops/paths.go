package fsops

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
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

const (
	// Highest C0 control character.
	controlCharMax = 0x1f

	// The C1 delete control character.
	deleteChar = 0x7f
)

// Contains the path separator characters on both Unix and Windows.
const pathSeparators = `/\`

// The path segment that traverses to the parent directory.
const parentSegment = ".."

// Reports whether path is safe to use.
// It rejects empty paths, NUL and other control characters (injection vectors), and ".." segments
// A dot sequence inside a segment name (e.g. "a..b") is legal and allowed.
func ValidatePath(path string) (err error) {
	if path == "" {
		err = fmt.Errorf("path is empty")
		return
	}

	for offset, char := range path {
		if char <= controlCharMax || char == deleteChar {
			err = fmt.Errorf("path contains control character 0x%02x at offset %d", char, offset)
			return
		}
	}

	if slices.Contains(strings.FieldsFunc(path, isPathSeparator), parentSegment) {
		err = fmt.Errorf("path %q contains a \"..\" traversal segment", path)
		return
	}

	return
}

// isPathSeparator reports whether r is a path separator on either platform.
func isPathSeparator(r rune) (isSep bool) {
	isSep = strings.ContainsRune(pathSeparators, r)
	return
}
