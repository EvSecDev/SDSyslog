package file

import (
	"os"
	"sdsyslog/internal/global"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	// set defaults
	var err error
	global.Hostname, err = os.Hostname()
	if err != nil {
		t.Fatalf("failed to determine local hostname: %v", err)
	}
	global.PID = os.Getpid()

	tests := []struct {
		name           string
		input          string
		expectedOutput global.ParsedMessage
	}{
		{
			name:  "Default",
			input: "short message",
			expectedOutput: global.ParsedMessage{
				Text:            "short message",
				ApplicationName: "-",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       time.Now(),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Default Empty",
			input: "",
			expectedOutput: global.ParsedMessage{
				Text:            "-",
				ApplicationName: "-",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       time.Now(),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 1",
			input: `Jul  9 18:05:33 Host1 rsyslogd: [origin software="rsyslogd" swVersion="8.2302.0" x-pid="4765" x-info="https://www.rsyslog.com"] start`,
			expectedOutput: global.ParsedMessage{
				Text:            `[origin software="rsyslogd" swVersion="8.2302.0" x-pid="4765" x-info="https://www.rsyslog.com"] start`,
				ApplicationName: "rsyslogd",
				Hostname:        "Host1",
				ProcessID:       global.PID,
				Timestamp:       timeParse1Panic("Jul  9 18:05:33"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 2",
			input: `Nov 17 09:52:41 Host1 kernel: Linux version 6.1.0-27-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (2024-11-01)`,
			expectedOutput: global.ParsedMessage{
				Text:            `Linux version 6.1.0-27-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (2024-11-01)`,
				ApplicationName: "kernel",
				Hostname:        "Host1",
				ProcessID:       global.PID,
				Timestamp:       timeParse1Panic("Nov 17 09:52:41"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 3",
			input: `Nov 17 12:18:00 Host1 audisp-syslog[1135]: type=BPF msg=audit(1731874680.879:116): prog-id=17 op=UNLOAD`,
			expectedOutput: global.ParsedMessage{
				Text:            `type=BPF msg=audit(1731874680.879:116): prog-id=17 op=UNLOAD`,
				ApplicationName: "audisp-syslog",
				Hostname:        "Host1",
				ProcessID:       1135,
				Timestamp:       timeParse1Panic("Nov 17 12:18:00"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 4",
			input: `2025/03/15 10:47:59 [notice] 33709#33709: using inherited sockets from "5;6;"`,
			expectedOutput: global.ParsedMessage{
				Text:            `using inherited sockets from "5;6;"`,
				ApplicationName: "-",
				Hostname:        global.Hostname,
				ProcessID:       33709,
				Timestamp:       timeParse2Panic("2025/03/15 10:47:59"),
				Facility:        global.DefaultFacility,
				Severity:        "notice",
			},
		},
		{
			name:  "Format Type 5",
			input: `2025-12-03 17:46:26 status triggers-pending libc-bin:amd64 2.41-12`,
			expectedOutput: global.ParsedMessage{
				Text:            `status triggers-pending libc-bin:amd64 2.41-12`,
				ApplicationName: "-",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       timeParse3Panic("2025-12-03 17:46:26"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 6",
			input: `10.10.10.10 - - [28/Jul/2024:03:58:35 -0700] "GET / HTTP/1.1" 444 0 "-" "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0"`,
			expectedOutput: global.ParsedMessage{
				Text:            `10.10.10.10 - - [28/Jul/2024:03:58:35 -0700] "GET / HTTP/1.1" 444 0 "-" "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0"`,
				ApplicationName: "-",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       timeParse5Panic("28/Jul/2024:03:58:35 -0700"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 7",
			input: `[19-Sep-2023 16:52:51] NOTICE: Terminating ...`,
			expectedOutput: global.ParsedMessage{
				Text:            `Terminating ...`,
				ApplicationName: "-",
				Hostname:        global.Hostname,
				ProcessID:       global.PID,
				Timestamp:       timeParse4Panic("19-Sep-2023 16:52:51"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 8",
			input: `2025-12-21T18:39:01.211585-08:00 Host1 systemd[1]: Starting phpsessionclean.service - Clean php session files...`,
			expectedOutput: global.ParsedMessage{
				Text:            `Starting phpsessionclean.service - Clean php session files...`,
				ApplicationName: "systemd",
				Hostname:        "Host1",
				ProcessID:       1,
				Timestamp:       timeParse6Panic("2025-12-21T18:39:01.211585-08:00"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
		{
			name:  "Format Type 8 No pid",
			input: `2025-12-21T19:08:28.506905-08:00 Host1 php8.4-cgi: php_invoke mbstring: already enabled for PHP 8.4 cgi sapi`,
			expectedOutput: global.ParsedMessage{
				Text:            `php_invoke mbstring: already enabled for PHP 8.4 cgi sapi`,
				ApplicationName: "php8.4-cgi",
				Hostname:        "Host1",
				ProcessID:       global.PID,
				Timestamp:       timeParse6Panic("2025-12-21T19:08:28.506905-08:00"),
				Facility:        global.DefaultFacility,
				Severity:        global.DefaultSeverity,
			},
		},
	}

	for _, tt := range tests {
		before := time.Now()
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			output := parseLine(tt.input)
			after := time.Now()
			if output.ApplicationName != tt.expectedOutput.ApplicationName {
				t.Errorf("expected ApplicationName to be '%s', but got '%s'", tt.expectedOutput.ApplicationName, output.ApplicationName)
			}
			if output.Facility != tt.expectedOutput.Facility {
				t.Errorf("expected Facility to be '%s', but got '%s'", tt.expectedOutput.Facility, output.Facility)
			}
			if output.Hostname != tt.expectedOutput.Hostname {
				t.Errorf("expected Hostname to be '%s', but got '%s'", tt.expectedOutput.Hostname, output.Hostname)
			}
			if output.ProcessID != tt.expectedOutput.ProcessID {
				t.Errorf("expected ProcessID to be '%d', but got '%d'", tt.expectedOutput.ProcessID, output.ProcessID)
			}
			if output.Severity != tt.expectedOutput.Severity {
				t.Errorf("expected Severity to be '%s', but got '%s'", tt.expectedOutput.Severity, output.Severity)
			}
			if output.Text != tt.expectedOutput.Text {
				t.Errorf("expected Text to be '%s', but got '%s'", tt.expectedOutput.Text, output.Text)
			}
			if output.Timestamp != tt.expectedOutput.Timestamp {
				if output.Timestamp.Before(before) || output.Timestamp.After(after) {
					t.Errorf("expected Timestamp to be '%s', but got '%s'", tt.expectedOutput.Timestamp, output.Timestamp)
				}
			}
		})
	}
}

// Parse no year timestamp log prefix
func timeParse1Panic(val string) (res time.Time) {
	res, err := time.Parse("Jan _2 15:04:05", val)
	if err != nil {
		panic(err)
	}
	res = withCurrentYear(res)
	return
}
func timeParse2Panic(val string) (res time.Time) {
	res, err := time.Parse("2006/01/02 15:04:05", val)
	if err != nil {
		panic(err)
	}
	return
}
func timeParse3Panic(val string) (res time.Time) {
	res, err := time.Parse("2006-01-02 15:04:05", val)
	if err != nil {
		panic(err)
	}
	return
}
func timeParse4Panic(val string) (res time.Time) {
	res, err := time.Parse("02-Jan-2006 15:04:05", val)
	if err != nil {
		panic(err)
	}
	return
}
func timeParse5Panic(val string) (res time.Time) {
	res, err := time.Parse("02/Jan/2006:15:04:05 -0700", val)
	if err != nil {
		panic(err)
	}
	return
}
func timeParse6Panic(val string) (res time.Time) {
	res, err := time.Parse("2006-01-02T15:04:05.999999-07:00", val)
	if err != nil {
		panic(err)
	}
	return
}
