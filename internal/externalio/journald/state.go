package journald

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getLastPosition(stateFilePath string) (cursor string, err error) {
	stateDirectory := filepath.Dir(stateFilePath)

	_, err = os.Stat(stateDirectory)
	if os.IsNotExist(err) {
		err = os.MkdirAll(stateDirectory, 0700)
		if err != nil {
			err = fmt.Errorf("failed to create missing state directory '%s': %w", stateDirectory, err)
			return
		}
	} else if err != nil {
		err = fmt.Errorf("unable to access state directory: %w", err)
		return
	}

	stateFile, err := os.OpenFile(stateFilePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		err = fmt.Errorf("failed to open state file: %w", err)
		return
	}
	defer stateFile.Close()

	// Retrieve cached data
	data := make([]byte, 256)
	n, err := stateFile.Read(data)
	if err != nil && err.Error() != "EOF" {
		err = fmt.Errorf("unable to read position file: %w", err)
		return
	} else {
		err = nil
	}
	cursor = string(data[:n])
	cursor = strings.Trim(cursor, "\n")

	// Validate cursor format - restart from zero otherwise
	testCursorFields := strings.Split(cursor, ";")
	if len(testCursorFields) < 3 {
		// Just checking to see if there are more than one (2 times could be a coincidence)
		cursor = ""
	}
	if strings.HasPrefix(testCursorFields[0], "s=") {
		cursor = ""
	}

	return
}

func savePosition(cursor string, stateFilePath string) (err error) {
	// Don't nuke existing cursor
	if cursor == "" {
		return
	}

	stateDirectory := filepath.Dir(stateFilePath)

	_, err = os.Stat(stateDirectory)
	if os.IsNotExist(err) {
		err = os.MkdirAll(stateDirectory, 0700)
		if err != nil {
			err = fmt.Errorf("failed to create missing state directory '%s': %w", stateDirectory, err)
			return
		}
	} else if err != nil {
		err = fmt.Errorf("unable to access state directory: %w", err)
		return
	}

	stateFile, err := os.OpenFile(stateFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		err = fmt.Errorf("failed to open state file: %w", err)
		return
	}
	defer stateFile.Close()

	_, err = fmt.Fprintf(stateFile, "%s", cursor)
	if err != nil {
		err = fmt.Errorf("failed to write current log position to state file: %w", err)
		return
	}
	return
}
