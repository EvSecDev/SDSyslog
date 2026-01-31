package file

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
)

// Creates new file input module. Returns nil nil if no path.
func NewInput(namespace []string, filePath string, baseStateFile string, queue *mpmc.Queue[protocol.Message]) (module *InModule, err error) {
	if filePath == "" {
		return
	}

	// Create unique state file for this source
	stateFileDir := filepath.Dir(baseStateFile)
	stateFileName := filepath.Base(baseStateFile)

	newStateFileName := base64.RawURLEncoding.EncodeToString([]byte(filePath)) + "_" + stateFileName // Using full file path as prefix to state file
	newStateFile := filepath.Join(stateFileDir, newStateFileName)

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		err = fmt.Errorf("failed to open source file: %v", err)
		return
	}

	module = &InModule{
		Namespace: namespace,
		sink:      file,
		filePath:  filePath,
		stateFile: newStateFile,
		outbox:    queue,
		metrics:   MetricStorage{},
	}

	return
}

// Creates new file output module. Returns nil nil if no path.
func NewOutput(filePath string) (module *OutModule, err error) {
	if filePath == "" {
		return
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return
	}

	module = &OutModule{
		sink:        file,
		batchBuffer: &[]string{},
	}
	return
}
