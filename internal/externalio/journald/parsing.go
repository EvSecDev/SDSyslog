package journald

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"
)

// Reads a journald export entry until complete entry (double newline).
// https://systemd.io/JOURNAL_EXPORT_FORMATS/#journal-export-format
func ExtractEntry(reader *bufio.Reader) (fields map[string]string, err error) {
	fields = make(map[string]string)
	for {
		var line string

		line, err = reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				err = nil
				// EOF means journal did not start properly
				// Sleeping longer than stderr check read timeout
				//   so if there was an error, we will return from this when the daemon is shutting down
				time.Sleep(30 * time.Millisecond)
				return
			} else {
				// Any other error
				err = fmt.Errorf("failed initial line read: %v", err)
				return
			}
		}
		line = strings.TrimSuffix(line, "\n")

		// End of entry when we hit empty (i.e. double newline after read + trim)
		if line == "" {
			break
		}

		// Text field
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				err = fmt.Errorf("invalid text field format: '%s'", line)
				return
			}

			fields[parts[0]] = parts[1]
			continue // next field
		}

		// Binary field
		// Data until newline is the key name
		key := line
		if key == "" {
			err = fmt.Errorf("invalid binary field: empty key")
			return
		}

		// Next 64 bits are the little-endian length field
		lenField := make([]byte, 8)
		_, err = io.ReadFull(reader, lenField)
		if err != nil {
			err = fmt.Errorf("failed binary field length read: %v", err)
			return
		}

		size := binary.LittleEndian.Uint64(lenField)

		// Sanity limit for binary fields (10MB)
		if size > 1024*1024*10 {
			err = fmt.Errorf("binary field size too large: %d bytes", size)
			return
		}

		// Read out the data of expected length
		data := make([]byte, size)
		_, err = io.ReadFull(reader, data)
		if err != nil {
			err = fmt.Errorf("failed binary field value read: %v", err)
			return
		}

		// Consume exactly one newline
		b, _ := reader.ReadByte()
		if b != '\n' {
			// Bail because field terminators will be off otherwise (causing panic at length field read above)
			err = fmt.Errorf("binary field missing newline")
			return
		}

		fields[key] = string(data)
	}

	if len(fields) == 0 {
		err = fmt.Errorf("encountered empty entry")
		return
	}

	return
}
