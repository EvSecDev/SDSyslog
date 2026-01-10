package out

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/output"
	"time"
)

// Create and start new output instance
func (manager *InstanceManager) AddInstance(filePath string, journaldURL string) (err error) {
	if filePath == "" && journaldURL == "" {
		err = fmt.Errorf("no outputs enabled/configured")
		return
	}

	// Create new context for output instance
	workerCtx, cancelInstance := context.WithCancel(context.Background())
	workerCtx = context.WithValue(workerCtx, global.LoggerKey, logctx.GetLogger(manager.ctx))

	instance := &OutputInstance{
		Worker: output.New(logctx.GetTagList(manager.ctx), manager.Queue),
		cancel: cancelInstance,
	}

	manager.Instance = instance

	// Add outputs
	if filePath != "" {
		var file *os.File
		file, err = os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			return
		}

		instance.Worker.FileOut = file
	}
	if journaldURL != "" {
		transport := &http.Transport{
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			DisableKeepAlives:     false,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: -1, // Not supported by journal remote server
		}

		var baseURL *url.URL
		baseURL, err = url.Parse(journaldURL)
		if err != nil {
			err = fmt.Errorf("invalid journald URL: %v", err)
			return
		}
		messagePublishPath := &url.URL{Path: "upload"} // Only path accepted by the remote server
		instance.Worker.JrnlURL = baseURL.ResolveReference(messagePublishPath).String()

		instance.Worker.JrnlOut = &http.Client{
			Transport: transport,
			Timeout:   0, // no per-request timeout
		}

		testCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		var req *http.Request
		req, err = http.NewRequestWithContext(
			testCtx,
			http.MethodPost,
			journaldURL,
			bytes.NewReader(nil),
		)
		if err != nil {
			err = fmt.Errorf("failed to create test HTTP connection to journald: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/vnd.fdo.journal")

		var resp *http.Response
		resp, err = instance.Worker.JrnlOut.Do(req)
		if err != nil {
			err = fmt.Errorf("failed to test HTTP connection to journald: %v", err)
			return
		}
		resp.Body.Close()
	}

	// Start worker
	instance.wg.Add(1)
	go func() {
		defer instance.wg.Done()
		workerCtx := logctx.OverwriteCtxTag(workerCtx, instance.Worker.Namespace)
		instance.Worker.Run(workerCtx)
	}()
	return
}

// Shutdown existing file output instance
func (manager *InstanceManager) RemoveInstance() {
	if manager.Instance == nil {
		return
	}
	if manager.Instance.cancel != nil {
		manager.Instance.cancel()
	}
	manager.Instance.wg.Wait()

	if manager.Instance.Worker.FileOut != nil {
		manager.Instance.Worker.FileOut.Close()
	}
	if manager.Instance.Worker.JrnlOut != nil {
		manager.Instance.Worker.JrnlOut.CloseIdleConnections()
	}
}
