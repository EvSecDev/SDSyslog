package file

import (
	"context"
	"sdsyslog/pkg/protocol"
	"sort"
	"strings"
	"time"
)

// Writes log message and associated metadata in one line to configured file
func (mod *OutModule) Write(ctx context.Context, msg protocol.Payload) (linesWritten int, err error) {
	if mod == nil {
		return
	}

	newEntry, err := formatAsText(ctx, msg)
	if err != nil {
		return
	}

	// Always ensure outputs have only one trailing newline
	var lineParts []string
	if !strings.HasSuffix(newEntry, "\n") {
		lineParts = append(lineParts, newEntry+"\n")
	} else {
		lineParts = []string{newEntry}
	}
	newLine := strings.Join(lineParts, " ")

	// Buffer small amount to reorder and write in batches
	*mod.batchBuffer = append(*mod.batchBuffer, newLine)

	// Batch 20 at a time
	if len(*mod.batchBuffer) > 20 {
		linesWritten, err = mod.FlushBuffer()
		if err != nil {
			return
		}
	}

	return
}

// Flushes line buffer to the file
func (mod *OutModule) FlushBuffer() (flushedCnt int, err error) {
	if mod.batchBuffer == nil {
		return
	}

	if len(*mod.batchBuffer) == 0 {
		return
	}

	sort.Slice(*mod.batchBuffer, func(i, j int) bool {
		// Extract timestamp prefix (up to first space)
		getTime := func(s string) time.Time {
			ts := s
			if idx := strings.IndexByte(s, ' '); idx != -1 {
				ts = s[:idx]
			}
			t, err := time.Parse(time.RFC3339Nano, ts)
			if err != nil {
				return time.Time{} // zero time on error
			}
			return t
		}

		ti := getTime((*mod.batchBuffer)[i])
		tj := getTime((*mod.batchBuffer)[j])

		// Newest first, compare reverse
		return ti.After(tj)
	})

	for _, line := range *mod.batchBuffer {
		data := []byte(line)
		for len(data) > 0 {
			var n int
			n, err = mod.sink.Write(data)
			if err != nil {
				return
			}
			data = data[n:] // remove the bytes that were successfully written
		}
		flushedCnt++
	}

	// All writes succeeded, empty buffer
	*mod.batchBuffer = []string{}

	return
}
