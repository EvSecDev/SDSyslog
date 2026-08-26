package file

import (
	"context"
	"sdsyslog/pkg/protocol"
	"slices"
	"strings"
	"time"
)

// Writes log message and associated metadata in one line to configured file
func (mod *OutModule) Write(ctx context.Context, msg *protocol.Payload) (linesWritten int, err error) {
	if mod == nil {
		return
	}

	newEntry := formatAsText(ctx, msg)

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

	// Flush buffer if full
	if len(*mod.batchBuffer) > mod.batchSize {
		linesWritten, err = mod.FlushBuffer()
		if err != nil {
			return
		}
	}

	return
}

// Flushes line buffer to the file
func (mod *OutModule) FlushBuffer() (flushedCnt int, err error) {
	if mod == nil {
		return
	}

	if mod.batchBuffer == nil {
		return
	}

	if len(*mod.batchBuffer) == 0 {
		return
	}

	slices.SortFunc(*mod.batchBuffer, func(lineA, lineB string) int {
		// Extract timestamp prefix (up to first space)
		getTime := func(timestamp string) time.Time {
			before, _, ok := strings.Cut(timestamp, " ")
			if ok {
				timestamp = before
			}
			ts, err := time.Parse(time.RFC3339Nano, timestamp)
			if err != nil {
				return time.Time{} // zero time on error
			}
			return ts
		}

		timeA := getTime(lineA)
		timeB := getTime(lineB)

		// Oldest first
		return timeA.Compare(timeB)
	})

	for _, line := range *mod.batchBuffer {
		data := []byte(line)
		for len(data) > 0 {
			var bytesRead int
			bytesRead, err = mod.sink.Write(data)
			if err != nil {
				return
			}
			data = data[bytesRead:] // remove the bytes that were successfully written
		}
		flushedCnt++
	}

	// All writes succeeded, empty buffer
	*mod.batchBuffer = []string{}

	return
}
