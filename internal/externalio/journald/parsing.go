package journald

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
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
				// EOF with no fields is error
				if len(fields) == 0 {
					err = fmt.Errorf("encountered empty entry with EOF")
					return
				}
				// EOF with fields is fine
				err = nil
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

// Extracts main cursor string position from cursor field (if present)
func ExtractCursor(allFields map[string]string) (cursor string, err error) {
	fullCursor, ok := allFields["__CURSOR"]
	if !ok {
		err = fmt.Errorf("entry did not contain cursor field")
		return
	} else {
		cursorFields := strings.Split(fullCursor, ";")
		rawCursorPosition := cursorFields[0]
		positionFields := strings.Split(rawCursorPosition, "=")
		if len(positionFields) > 1 {
			if positionFields[0] != "s" {
				err = fmt.Errorf("first cursor field is not main identification string")
				return
			}
			if positionFields[1] == "" {
				err = fmt.Errorf("main cursor field is empty")
				return
			}
			cursor = positionFields[1]
		} else {
			err = fmt.Errorf("no 's' field inside string '%s'", fullCursor)
			return
		}
	}
	return
}
