package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Retrieve last read position for the log file from the state file
func GetLastPosition(logFilePath string, stateFilePath string) (inode uint64, position int64, err error) {
	stateDirectory := filepath.Dir(stateFilePath)

	_, err = os.Stat(stateDirectory)
	if os.IsNotExist(err) {
		err = os.MkdirAll(stateDirectory, 0700)
		if err != nil {
			err = fmt.Errorf("failed to create missing state directory '%s': %v", stateDirectory, err)
			return
		}
	} else if err != nil {
		err = fmt.Errorf("unable to access state directory: %v", err)
		return
	}

	stateFile, err := os.OpenFile(stateFilePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			inode, position = 0, 0
			return
		}
		err = fmt.Errorf("failed to open state file: %v", err)
		return
	}
	defer stateFile.Close()

	// Retrieve cached data
	data := make([]byte, 128)
	n, err := stateFile.Read(data)
	if err != nil && err.Error() != "EOF" {
		err = fmt.Errorf("unable to read position file: %v", err)
		return
	} else {
		err = nil
	}

	content := strings.TrimSpace(string(data[:n]))
	if content == "" {
		// Empty state file, assume new
		return
	}

	parts := strings.Fields(content)
	if len(parts) != 2 {
		// Remove any invalid state data in state file
		err = stateFile.Truncate(0)
		if err != nil {
			fmt.Printf("Error truncating file: %v\n", err)
			return
		}

		return
	}

	inodeParsed, err1 := strconv.ParseUint(parts[0], 10, 64)
	posParsed, err2 := strconv.ParseInt(parts[1], 10, 64)
	if len(parts) != 2 || err1 != nil || err2 != nil {
		// Remove any invalid state data in state file
		err = stateFile.Truncate(0)
		if err != nil {
			fmt.Printf("Error truncating file: %v\n", err)
			return
		}

		return
	}

	fileInfo, err := os.Stat(logFilePath)
	if err != nil {
		err = fmt.Errorf("unable to stat log file: %v", err)
		return
	}

	// Avoid offsets beyond end of file
	fileSize := fileInfo.Size()
	if position > fileSize {
		position = fileSize
	}

	// Avoid using cached offsets if inode is not current
	stat := fileInfo.Sys().(*syscall.Stat_t)
	currentInode := stat.Ino

	// If inode matches, return cached offset, else reset offset to 0
	if inodeParsed == currentInode {
		inode = inodeParsed
		position = posParsed
	} else {
		inode = currentInode
		position = 0
	}

	return
}

// Save the current file read position to the state file
func SavePosition(stateFilePath string, inode uint64, position int64) (err error) {
	stateDirectory := filepath.Dir(stateFilePath)

	_, err = os.Stat(stateDirectory)
	if os.IsNotExist(err) {
		err = os.MkdirAll(stateDirectory, 0700)
		if err != nil {
			err = fmt.Errorf("failed to create missing state directory '%s': %v", stateDirectory, err)
			return
		}
	} else if err != nil {
		err = fmt.Errorf("unable to access state directory: %v", err)
		return
	}

	stateFile, err := os.OpenFile(stateFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		err = fmt.Errorf("failed to open state file: %v", err)
		return
	}
	defer stateFile.Close()

	_, err = fmt.Fprintf(stateFile, "%d %d", inode, position)
	if err != nil {
		err = fmt.Errorf("failed to write current log position to state file: %v", err)
		return
	}
	return
}
