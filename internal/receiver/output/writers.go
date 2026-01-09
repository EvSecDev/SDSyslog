package output

import (
	"io"
	"sdsyslog/pkg/protocol"
	"sort"
	"strings"
	"time"
)

// Writes log message and associated metadata in one line to configured file
func writeFile(lineBuffer *[]string, msg protocol.Payload, file io.Writer) (err error) {
	newEntry := FormatAsText(msg)

	// Always ensure outputs have only one trailing newline
	var lineParts []string
	if !strings.HasSuffix(newEntry, "\n") {
		lineParts = append(lineParts, newEntry+"\n")
	} else {
		lineParts = []string{newEntry}
	}
	newLine := strings.Join(lineParts, " ")

	// Buffer small amount to reorder and write in batches
	*lineBuffer = append(*lineBuffer, newLine)

	// Batch 20 at a time
	if len(*lineBuffer) > 20 {
		err = flushFileBuffer(lineBuffer, file)
		if err != nil {
			return
		}
	}

	return
}

// Flushes line buffer to the file
func flushFileBuffer(lineBuffer *[]string, file io.Writer) (err error) {
	if lineBuffer == nil {
		return
	}

	if len(*lineBuffer) == 0 {
		return
	}

	sort.Slice(*lineBuffer, func(i, j int) bool {
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

		ti := getTime((*lineBuffer)[i])
		tj := getTime((*lineBuffer)[j])

		// Newest first, compare reverse
		return ti.After(tj)
	})

	for _, line := range *lineBuffer {
		data := []byte(line)
		for len(data) > 0 {
			var n int
			n, err = file.Write(data)
			if err != nil {
				return
			}
			data = data[n:] // remove the bytes that were successfully written
		}
	}

	// All writes succeeded, empty buffer
	*lineBuffer = []string{}

	return
}
