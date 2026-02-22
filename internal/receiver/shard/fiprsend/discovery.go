package fiprsend

import (
	"fmt"
	"os"
	"sdsyslog/internal/global"
	"sort"
	"strconv"
	"strings"
)

// Retrieves ordered (stable) socket file list from given directory.
// Excludes our own socket file for given PID.
func GetSocketFileList(socketDir string, selfID int) (fileList []string, err error) {
	entries, err := os.ReadDir(socketDir)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			// no sockets/access, nothing to do
			err = nil
			return
		}
		err = fmt.Errorf("failed to read socket directory: %w", err)
		return
	}

	selfSocketFile := global.SocketFileNamePrefix + strconv.Itoa(selfID) + global.SocketFileNameSuffix

	for _, entry := range entries {
		// Skip any normal files/dirs
		if entry.Type().IsRegular() || entry.Type().IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), global.SocketFileNameSuffix) {
			continue
		}
		if !strings.HasPrefix(entry.Name(), global.SocketFileNamePrefix) {
			continue
		}

		if entry.Name() != selfSocketFile {
			fileList = append(fileList, entry.Name())
		}
	}

	// Stable, deterministic sort
	sort.SliceStable(fileList, func(a, b int) bool {
		return fileList[a] < fileList[b]
	})
	return
}
