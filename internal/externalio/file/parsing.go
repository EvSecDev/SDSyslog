package file

import (
	"sdsyslog/internal/global"
	"sdsyslog/pkg/protocol"
	"strconv"
	"strings"
	"time"
)

// Parses file line text for common formats and extracts metadata. (The Monstrosity of Assumption TM)
func parseLine(rawLine string) (message global.ParsedMessage) {
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
func setDefaults(old global.ParsedMessage, raw string) (new global.ParsedMessage) {
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

// Main raw log line format for outputs
// Fmt: '2020-01-01T10:10:10.123456789Z Server01 MyApp[1234]: Daemon: [INFO]: this is a log message'
func formatAsText(msg protocol.Payload) (text string) {
	text =
		msg.Timestamp.Format(time.RFC3339Nano) + " " +
			msg.Hostname + " " +
			msg.ApplicationName +
			"[" + strconv.Itoa(msg.ProcessID) + "]: " +
			msg.Facility + ": " +
			"[" + strings.ToUpper(msg.Severity) + "]: " +
			string(msg.LogText)
	return
}
