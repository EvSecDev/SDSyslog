package journald

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// Writes journald export format byte payload to the journald-remote HTTP endpoint
func sendJournalExport(client *http.Client, url string, payload []byte) (err error) {
	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewReader(payload),
	)
	if err != nil {
		err = fmt.Errorf("failed request creation: %w", err)
		return
	}

	req.Header.Set("Content-Type", "application/vnd.fdo.journal") // journald export format
	req.Header.Del("Expect")                                      // Unsupported by journal remote server (will cause errors if set)

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed HTTP request: %w", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("received HTTP status '%s'", resp.Status)

		// Include response body if present for additional error details
		if resp.ContentLength > 0 {
			buf := make([]byte, resp.ContentLength)
			_, lerr := resp.Body.Read(buf)
			if lerr != nil && lerr != io.EOF {
				err = fmt.Errorf("%w: response body present (%d bytes) but read failed: %w", err, resp.ContentLength, lerr)
			} else {
				err = fmt.Errorf("%w: %s", err, string(buf))
			}
		}
		return
	}

	return
}
