package file

import (
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/pkg/protocol"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	// set defaults
	var err error
	localHostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("failed to determine local hostname: %v", err)
	}
	testPid := os.Getpid()

	tests := []struct {
		name           string
		input          string
		expectedOutput protocol.Message
	}{
		{
			name:  "Default",
			input: "short message",
			expectedOutput: protocol.Message{
				Data:      "short message",
				Hostname:  localHostname,
				Timestamp: time.Now(),
				Fields: map[string]any{
					global.CFappname:   "-",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Default Empty",
			input: "",
			expectedOutput: protocol.Message{
				Data:      "-",
				Hostname:  localHostname,
				Timestamp: time.Now(),
				Fields: map[string]any{
					global.CFappname:   "-",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 1",
			input: `Jul  9 18:05:33 Host1 rsyslogd: [origin software="rsyslogd" swVersion="8.2302.0" x-pid="4765" x-info="https://www.rsyslog.com"] start`,
			expectedOutput: protocol.Message{
				Data:      `[origin software="rsyslogd" swVersion="8.2302.0" x-pid="4765" x-info="https://www.rsyslog.com"] start`,
				Hostname:  "Host1",
				Timestamp: timeParse1Panic("Jul  9 18:05:33"),
				Fields: map[string]any{
					global.CFappname:   "rsyslogd",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 2",
			input: `Nov 17 09:52:41 Host1 kernel: Linux version 6.1.0-27-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (2024-11-01)`,
			expectedOutput: protocol.Message{
				Data:      `Linux version 6.1.0-27-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (2024-11-01)`,
				Hostname:  "Host1",
				Timestamp: timeParse1Panic("Nov 17 09:52:41"),
				Fields: map[string]any{
					global.CFappname:   "kernel",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 3",
			input: `Nov 17 12:18:00 Host1 audisp-syslog[1135]: type=BPF msg=audit(1731874680.879:116): prog-id=17 op=UNLOAD`,
			expectedOutput: protocol.Message{
				Data:      `type=BPF msg=audit(1731874680.879:116): prog-id=17 op=UNLOAD`,
				Hostname:  "Host1",
				Timestamp: timeParse1Panic("Nov 17 12:18:00"),
				Fields: map[string]any{
					global.CFappname:   "audisp-syslog",
					global.CFprocessid: 1135,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 4",
			input: `2025/03/15 10:47:59 [notice] 33709#33709: using inherited sockets from "5;6;"`,
			expectedOutput: protocol.Message{
				Data:      `using inherited sockets from "5;6;"`,
				Hostname:  localHostname,
				Timestamp: timeParse2Panic("2025/03/15 10:47:59"),
				Fields: map[string]any{
					global.CFappname:   "-",
					global.CFprocessid: 33709,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  "notice",
				},
			},
		},
		{
			name:  "Format Type 5",
			input: `2025-12-03 17:46:26 status triggers-pending libc-bin:amd64 2.41-12`,
			expectedOutput: protocol.Message{
				Data:      `status triggers-pending libc-bin:amd64 2.41-12`,
				Hostname:  localHostname,
				Timestamp: timeParse3Panic("2025-12-03 17:46:26"),
				Fields: map[string]any{
					global.CFappname:   "-",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 6",
			input: `10.10.10.10 - - [28/Jul/2024:03:58:35 -0700] "GET / HTTP/1.1" 444 0 "-" "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0"`,
			expectedOutput: protocol.Message{
				Data:      `10.10.10.10 - - [28/Jul/2024:03:58:35 -0700] "GET / HTTP/1.1" 444 0 "-" "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0"`,
				Hostname:  localHostname,
				Timestamp: timeParse5Panic("28/Jul/2024:03:58:35 -0700"),
				Fields: map[string]any{
					global.CFappname:   "-",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 7",
			input: `[19-Sep-2023 16:52:51] NOTICE: Terminating ...`,
			expectedOutput: protocol.Message{
				Data:      `Terminating ...`,
				Hostname:  localHostname,
				Timestamp: timeParse4Panic("19-Sep-2023 16:52:51"),
				Fields: map[string]any{
					global.CFappname:   "-",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 8",
			input: `2025-12-21T18:39:01.211585-08:00 Host1 systemd[1]: Starting phpsessionclean.service - Clean php session files...`,
			expectedOutput: protocol.Message{
				Data:      `Starting phpsessionclean.service - Clean php session files...`,
				Hostname:  "Host1",
				Timestamp: timeParse6Panic("2025-12-21T18:39:01.211585-08:00"),
				Fields: map[string]any{
					global.CFappname:   "systemd",
					global.CFprocessid: 1,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 8 No pid",
			input: `2025-12-21T19:08:28.506905-08:00 Host1 php8.4-cgi: php_invoke mbstring: already enabled for PHP 8.4 cgi sapi`,
			expectedOutput: protocol.Message{
				Data:      `php_invoke mbstring: already enabled for PHP 8.4 cgi sapi`,
				Hostname:  "Host1",
				Timestamp: timeParse6Panic("2025-12-21T19:08:28.506905-08:00"),
				Fields: map[string]any{
					global.CFappname:   "php8.4-cgi",
					global.CFprocessid: testPid,
					global.CFfacility:  global.DefaultFacility,
					global.CFseverity:  global.DefaultSeverity,
				},
			},
		},
	}

	for _, tt := range tests {
		before := time.Now()
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			output := parseLine(tt.input, localHostname)
			after := time.Now()
			if output.Data != tt.expectedOutput.Data {
				t.Errorf("expected Data to be '%s', but got '%s'", tt.expectedOutput.Data, output.Data)
			}
			if output.Timestamp != tt.expectedOutput.Timestamp {
				if output.Timestamp.Before(before) || output.Timestamp.After(after) {
					t.Errorf("expected Timestamp to be '%s', but got '%s'", tt.expectedOutput.Timestamp, output.Timestamp)
				}
			}
			if output.Hostname != tt.expectedOutput.Hostname {
				t.Errorf("expected Hostname to be '%s', but got '%s'", tt.expectedOutput.Hostname, output.Hostname)
			}

			expected := tt.expectedOutput.Fields[global.CFappname]
			got, ok := output.Fields[global.CFappname]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFappname)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFappname, expected, got)
			}

			expected = tt.expectedOutput.Fields[global.CFfacility]
			got, ok = output.Fields[global.CFfacility]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFfacility)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFfacility, expected, got)
			}

			expected = tt.expectedOutput.Fields[global.CFprocessid]
			got, ok = output.Fields[global.CFprocessid]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFprocessid)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFprocessid, expected, got)
			}

			expected = tt.expectedOutput.Fields[global.CFseverity]
			got, ok = output.Fields[global.CFseverity]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", global.CFseverity)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", global.CFseverity, expected, got)
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
