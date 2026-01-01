// Watches and reads lines from configured sources
package listener

import (
	"context"
	"io"
	"os"
	"sdsyslog/internal/externalio/file"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// New creates a file listener instance
func NewFileSource(namespace []string, filePath string, stateFilePath string, queue *mpmc.Queue[ParsedMessage]) (new *FileInstance) {
	new = &FileInstance{
		Namespace:       append(namespace, global.NSoFile),
		Outbox:          queue,
		SourceFilePath:  filePath,
		SourceStateFile: stateFilePath,
		Metrics:         &MetricStorage{},
	}
	return
}

func (instance *FileInstance) Run(ctx context.Context) {
	logFileInode, logFileOffset, err := file.GetLastPosition(instance.SourceFilePath, instance.SourceStateFile)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "failed to get position of last source file read for '%s': %v\n", instance.SourceFilePath, err)
	}

	_, err = instance.Source.Seek(logFileOffset, io.SeekStart)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to resume last source file read position for '%s': %v\n", instance.SourceFilePath, err)
	}

	// Create inotify background watcher
	fileHasChanged := make(chan bool, 1) // Main blocker for reading new lines
	fileHasRotated := make(chan bool, 1) // Notify when to switch file inode and reset offset
	go file.Watcher(ctx, instance.SourceFilePath, fileHasChanged, fileHasRotated)

	buf := make([]byte, 65536)
	lineBuf := []byte{} // note: unbounded

	for {
		for {
			// Record current file position before read
			startOfChunk, err := instance.Source.Seek(0, io.SeekCurrent)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to retrieve current position in log file: %v\n", err)
			}

			n, err := instance.Source.Read(buf)
			if n == 0 {
				// no more bytes available, break to outer select for blocking
				break
			} else if err == io.EOF {
				// no more bytes available, break to outer select for blocking
				break
			} else if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "read error: %v", err)
				break
			}

			// process the bytes read
			offset := startOfChunk
			for i := 0; i < n; i++ {
				b := buf[i]

				if b != byte('\n') {
					// Regular line characters, add to buffer
					lineBuf = append(lineBuf, b)
					offset++
					continue
				}

				// line complete, process it
				instance.Metrics.LinesRead.Add(1)

				msg := parseLine(string(lineBuf))

				pushBlocking(ctx, instance.Outbox, msg)
				instance.Metrics.Success.Add(1)

				// reset line buffer
				lineBuf = []byte{}
				offset++ // move past newline
			}
			logFileOffset = offset

			// if line was complete, break inner loop to block
			if len(lineBuf) == 0 {
				break
			}
		}

		// Block until file change, file rotation, or cancellation
		select {
		case <-ctx.Done():
			err := file.SavePosition(instance.SourceStateFile, logFileInode, logFileOffset)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed to save position in file source '%s': %v\n", instance.SourceFilePath, err)
			}
			return
		case <-fileHasChanged:
			// file changed, continue scanning
		case reopenLogFile := <-fileHasRotated:
			if reopenLogFile {
				// Reopen at file path
				instance.Source.Close()
				instance.Source, err = os.Open(instance.SourceFilePath)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to reopen rotated log file: %v\n", err)
					continue
				}

				// Retrieve new file inode
				fileInfo, err := os.Stat(instance.SourceFilePath)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "unable to stat new source file: %v\n", err)
					continue
				}
				stat := fileInfo.Sys().(*syscall.Stat_t)

				// Save new file inode to state var
				logFileInode = stat.Ino

				// Reset offset position for new file
				logFileOffset = 0
			}
		}

		// Re-scan for new lines after the last offset
		_, err = instance.Source.Seek(logFileOffset, io.SeekStart)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "failed to seek to last offset: %v\n", err)
		}
	}

}

// Parses file line text for common formats and extracts metadata. (The Monstrosity of Assumption TM)
func parseLine(rawLine string) (message ParsedMessage) {
	line := strings.TrimSpace(rawLine)

	// Format: Syslog
	if len(line) >= 15 {
		ts, err := time.Parse("Jan _2 15:04:05", line[:15])
		if err == nil {
			rest := strings.TrimSpace(line[15:])

			// host
			hostEnd := strings.IndexByte(rest, ' ')
			if hostEnd > 0 {
				message.Hostname = rest[:hostEnd]
				rest = strings.TrimSpace(rest[hostEnd+1:])

				// app[:pid]:
				colon := strings.Index(rest, ":")
				if colon > 0 {
					header := rest[:colon]
					message.Text = strings.TrimSpace(rest[colon+1:])

					// app[pid] or app
					if lb := strings.IndexByte(header, '['); lb > 0 {
						message.ApplicationName = header[:lb]
						if rb := strings.IndexByte(header, ']'); rb > lb+1 {
							if pid, err := strconv.Atoi(header[lb+1 : rb]); err == nil {
								message.ProcessID = pid
							}
						}
					} else {
						message.ApplicationName = header
					}

					message.Timestamp = withCurrentYear(ts)
					message = setDefaults(message, line)
					return
				}
			}
		}
	}

	// Format: Syslog 2
	if len(line) >= 33 && line[10] == 'T' { // Check for the ISO8601 timestamp format
		tsStr := line[:32] // Extract the timestamp part

		// Parse the timestamp
		ts, err := time.Parse("2006-01-02T15:04:05.999999-07:00", tsStr)
		if err == nil {
			message.Timestamp = ts
			rest := strings.TrimSpace(line[32:])

			// Extract Hostname (before first space)
			hostEnd := strings.IndexByte(rest, ' ')
			if hostEnd > 0 {
				message.Hostname = rest[:hostEnd]
				rest = strings.TrimSpace(rest[hostEnd+1:]) // Get the remaining part after the hostname

				// Extract ApplicationName and ProcessID if present
				pidStart := strings.Index(rest, "[")
				pidEnd := strings.Index(rest, "]")
				if pidStart > 0 && pidEnd > pidStart {
					// Process includes PID in square brackets
					message.ApplicationName = rest[:pidStart]
					pidStr := rest[pidStart+1 : pidEnd]

					// Convert PID to an integer
					if pid, err := strconv.Atoi(pidStr); err == nil {
						message.ProcessID = pid
					}

					// Extract the message text after the PID part
					rest = strings.TrimPrefix(rest[pidEnd+1:], ":")
				} else {
					// No PID, extract ApplicationName before the colon
					colonIndex := strings.Index(rest, ":")
					if colonIndex > 0 {
						message.ApplicationName = rest[:colonIndex]
						rest = rest[colonIndex+1:] // Everything after the colon is the message text
					}
				}
				rest = strings.TrimSpace(rest)

				// Remaining part is the message text
				message.Text = rest
			}
			message = setDefaults(message, line)
			return
		}
	}

	// Format: nginx
	if len(line) >= 19 {
		ts, err := time.Parse("2006/01/02 15:04:05", line[:19])
		if err == nil {
			rest := strings.TrimSpace(line[19:])

			if strings.HasPrefix(rest, "[") {
				if rb := strings.Index(rest, "]"); rb > 1 {
					message.Severity = strings.ToLower(rest[1:rb])
					rest = strings.TrimSpace(rest[rb+1:])

					if hash := strings.Index(rest, "#"); hash > 0 {
						if colon := strings.Index(rest, ":"); colon > hash {
							if pid, err := strconv.Atoi(rest[:hash]); err == nil {
								message.ProcessID = pid
							}
							message.Text = strings.TrimSpace(rest[colon+1:])
							message.Timestamp = ts
							message = setDefaults(message, line)
							return
						}
					}
				}
			}
		}
	}

	// Format: Debian dpkg
	if len(line) >= 19 {
		if ts, err := time.Parse("2006-01-02 15:04:05", line[:19]); err == nil {
			message.Timestamp = ts
			message.Text = strings.TrimSpace(line[19:])
			message = setDefaults(message, line)
			return
		}
	}

	// Format: Apache access log
	if lb := strings.Index(line, "["); lb >= 0 {
		if rb := strings.Index(line[lb:], "]"); rb > 0 {
			tsStr := line[lb+1 : lb+rb]
			if ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", tsStr); err == nil {
				message.Timestamp = ts
			}
		}
	}

	// Format: PHP
	if strings.HasPrefix(line, "[") {
		if rb := strings.Index(line, "]"); rb > 0 {
			tsStr := line[1:rb]
			if ts, err := time.Parse("02-Jan-2006 15:04:05", tsStr); err == nil {
				rest := strings.TrimSpace(line[rb+1:])
				if colon := strings.Index(rest, ":"); colon > 0 {
					message.Text = strings.TrimSpace(rest[colon+1:])
				}
				message.Timestamp = ts
				message = setDefaults(message, line)
				return
			}
		}
	}

	message = setDefaults(message, line)
	return
}

// Adds year (and timezone) to timestamps that do not have one
func withCurrentYear(old time.Time) (new time.Time) {
	now := time.Now()
	new = time.Date(
		now.Year(),
		old.Month(),
		old.Day(),
		old.Hour(),
		old.Minute(),
		old.Second(),
		0,
		time.Local,
	)
	return
}

// Replaces empty fields with expected defaults
func setDefaults(old ParsedMessage, raw string) (new ParsedMessage) {
	new = old
	if new.ApplicationName == "" {
		new.ApplicationName = "-"
	}
	if new.Hostname == "" {
		new.Hostname = global.Hostname
	}
	if new.ProcessID == 0 {
		new.ProcessID = global.PID
	}
	if new.Timestamp.IsZero() {
		new.Timestamp = time.Now()
	}
	if new.Facility == "" {
		new.Facility = global.DefaultFacility
	}
	if new.Severity == "" {
		new.Severity = global.DefaultSeverity
	}
	if new.Text == "" {
		if raw == "" {
			raw = "-"
		}
		new.Text = raw
	}
	return
}
