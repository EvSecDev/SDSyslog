package journald

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"sdsyslog/internal/global"
	"sdsyslog/internal/queue/mpmc"
	"time"
)

// Creates new journald listener module
func NewInput(namespace []string, baseStateFile string, queue *mpmc.Queue[global.ParsedMessage]) (new *InModule, err error) {
	// Create unique state file for journal
	stateFileDir := filepath.Dir(baseStateFile)
	stateFileName := filepath.Base(baseStateFile)

	newStateFileName := "journal_" + stateFileName
	newStateFile := filepath.Join(stateFileDir, newStateFileName)

	// Load last cursor
	oldPos, err := getLastPosition(newStateFile)
	if err != nil {
		return
	}

	// Set the cursor position to the last known position or default to empty for the beginning
	var readPosition string
	if oldPos != "" {
		readPosition = oldPos
	} else {
		// Default to beginning if no cursor is available
		readPosition = ""
	}

	// Journal command args
	cmdArgs := []string{"--output=export", "--follow", "--no-pager"}

	if readPosition != "" {
		// Add the cursor flag to resume from the last position
		cmdArgs = append(cmdArgs, "--after-cursor", readPosition)
	}

	// Journal command
	jrnlCmd := exec.Command("journalctl", cmdArgs...)
	stdout, err := jrnlCmd.StdoutPipe()
	if err != nil {
		err = fmt.Errorf("failed to create stdout pipe for journalctl command: %v", err)
		return
	}
	stderr, err := jrnlCmd.StderrPipe()
	if err != nil {
		err = fmt.Errorf("failed to create stderr pipe for journalctl command: %v", err)
		return
	}

	new = &InModule{
		Namespace: append(namespace, global.NSoJrnl),
		cmd:       jrnlCmd,
		sink:      stdout,
		err:       stderr,
		stateFile: newStateFile,
		outbox:    queue,
		metrics:   MetricStorage{},
	}
	return
}

// Creates new journald output module. Tests connection. Returns nil nil if no url.
func NewOutput(endpoint string) (module *OutModule, err error) {
	if endpoint == "" {
		return
	}

	new := &OutModule{}

	transport := &http.Transport{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: -1, // Not supported by journal remote server
	}

	var baseURL *url.URL
	baseURL, err = url.Parse(endpoint)
	if err != nil {
		err = fmt.Errorf("invalid journald URL: %v", err)
		return
	}
	messagePublishPath := &url.URL{Path: "upload"} // Only path accepted by the remote server
	new.url = baseURL.ResolveReference(messagePublishPath).String()

	new.sink = &http.Client{
		Transport: transport,
		Timeout:   0, // no per-request timeout
	}

	testCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var req *http.Request
	req, err = http.NewRequestWithContext(
		testCtx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(nil),
	)
	if err != nil {
		err = fmt.Errorf("failed to create test HTTP connection to journald: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/vnd.fdo.journal")

	var resp *http.Response
	resp, err = new.sink.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to test HTTP connection to journald: %v", err)
		return
	}
	resp.Body.Close()

	module = new
	return
}
