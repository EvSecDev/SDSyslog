package journald

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sdsyslog/internal/logctx"
	"strings"
	"time"
)

func getLastPosition(ctx context.Context, stateFilePath string) (cursor string, err error) {
	stateDirectory := filepath.Dir(stateFilePath)

	_, err = os.Stat(stateDirectory)
	if os.IsNotExist(err) {
		err = os.MkdirAll(stateDirectory, 0o700)
		if err != nil {
			err = fmt.Errorf("failed to create missing state directory '%s': %w", stateDirectory, err)
			return
		}
	} else if err != nil {
		err = fmt.Errorf("unable to access state directory: %w", err)
		return
	}

	stateFile, err := os.OpenFile(stateFilePath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		err = fmt.Errorf("failed to open state file: %w", err)
		return
	}
	defer func() {
		_ = stateFile.Close()
	}()

	// Retrieve cached data
	data := make([]byte, 256)
	bytesRead, err := stateFile.Read(data)
	if err != nil && err.Error() != "EOF" {
		err = fmt.Errorf("unable to read position file: %w", err)
		return
	}
	err = nil

	cursor = string(data[:bytesRead])
	cursor = strings.Trim(cursor, "\n")

	// Validate cursor format - restart from zero otherwise
	testCursorFields := strings.Split(cursor, ";")

	// Just checking to see if there are more than one (2 times could be a coincidence)
	if len(testCursorFields) < 3 {
		logctx.LogStdWarn(ctx, "Found corrupted journal cursor in state file '%s': less than 3 fields\n", cursor)
		cursor = ""
	}
	// Ensure the cursor looks like a valid cursor
	if !strings.HasPrefix(testCursorFields[0], "s=") {
		logctx.LogStdWarn(ctx, "Found corrupted journal cursor in state file '%s': improper prefix\n", cursor)
		cursor = ""
	}

	return
}

func savePosition(cursor, stateFilePath string) (err error) {
	// Don't nuke existing cursor
	if cursor == "" {
		return
	}

	stateDirectory := filepath.Dir(stateFilePath)

	_, err = os.Stat(stateDirectory)
	if os.IsNotExist(err) {
		err = os.MkdirAll(stateDirectory, 0o700)
		if err != nil {
			err = fmt.Errorf("failed to create missing state directory '%s': %w", stateDirectory, err)
			return
		}
	} else if err != nil {
		err = fmt.Errorf("unable to access state directory: %w", err)
		return
	}

	stateFile, err := os.OpenFile(stateFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		err = fmt.Errorf("failed to open state file: %w", err)
		return
	}
	defer func() {
		_ = stateFile.Close()
	}()

	_, err = fmt.Fprintf(stateFile, "%s", cursor)
	if err != nil {
		err = fmt.Errorf("failed to write current log position to state file: %w", err)
		return
	}
	return
}

// Safely retrieves read position from atomic value - any error logs in context but returns empty cursor
func (mod *InModule) getCurrentReadPosition() (cursor string) {
	value := mod.currentPosition.Load()
	position, valid := value.(*string)
	if !valid {
		logctx.LogStdWarn(mod.ctx, "Attempt to retrieve last read position cursor returned not a string: got type %s\n",
			reflect.TypeFor[*string]())
		return
	}
	if position == nil {
		logctx.LogStdWarn(mod.ctx, "Attempt to retrieve last read position cursor returned a null string pointer\n")
		return
	}
	cursor = *position
	return
}

// Safely stores read position as atomic value from value - any error logs in context
func (mod *InModule) setCurrentReadPosition(cursor string) {
	mod.currentPosition.Store(&cursor)
}

// Ticker based loop to flush current read position to state file
func (mod *InModule) periodicPositionSaver(saveInterval time.Duration) {
	defer mod.wg.Done()

	ticker := time.NewTicker(saveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-mod.ctx.Done():
			return
		case <-ticker.C:
			readPosition := mod.getCurrentReadPosition()
			if readPosition == "" {
				continue
			}
			err := savePosition(readPosition, mod.stateFile)
			if err != nil {
				logctx.LogStdErr(mod.ctx, "Failed to save current read position cursor to state file: %w\n", err)
				continue
			}
		}
	}
}
