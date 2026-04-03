package file

import (
	"bytes"
	"context"
	"net/netip"
	"os"
	"sdsyslog/internal/iomodules"
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
				Data:      []byte("short message"),
				Hostname:  localHostname,
				Timestamp: time.Now(),
				Fields: map[string]any{
					iomodules.CFappname:   "-",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Default Empty",
			input: "",
			expectedOutput: protocol.Message{
				Data:      []byte("-"),
				Hostname:  localHostname,
				Timestamp: time.Now(),
				Fields: map[string]any{
					iomodules.CFappname:   "-",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 1",
			input: `Jul  9 18:05:33 Host1 rsyslogd: [origin software="rsyslogd" swVersion="8.2302.0" x-pid="4765" x-info="https://www.rsyslog.com"] start`,
			expectedOutput: protocol.Message{
				Data:      []byte(`[origin software="rsyslogd" swVersion="8.2302.0" x-pid="4765" x-info="https://www.rsyslog.com"] start`),
				Hostname:  "Host1",
				Timestamp: timeParse1Panic("Jul  9 18:05:33"),
				Fields: map[string]any{
					iomodules.CFappname:   "rsyslogd",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 2",
			input: `Nov 17 09:52:41 Host1 kernel: Linux version 6.1.0-27-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (2024-11-01)`,
			expectedOutput: protocol.Message{
				Data:      []byte(`Linux version 6.1.0-27-amd64 (debian-kernel@lists.debian.org) (gcc-12 (Debian 12.2.0-14) 12.2.0, GNU ld (2024-11-01)`),
				Hostname:  "Host1",
				Timestamp: timeParse1Panic("Nov 17 09:52:41"),
				Fields: map[string]any{
					iomodules.CFappname:   "kernel",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 3",
			input: `Nov 17 12:18:00 Host1 audisp-syslog[1135]: type=BPF msg=audit(1731874680.879:116): prog-id=17 op=UNLOAD`,
			expectedOutput: protocol.Message{
				Data:      []byte(`type=BPF msg=audit(1731874680.879:116): prog-id=17 op=UNLOAD`),
				Hostname:  "Host1",
				Timestamp: timeParse1Panic("Nov 17 12:18:00"),
				Fields: map[string]any{
					iomodules.CFappname:   "audisp-syslog",
					iomodules.CFprocessid: 1135,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 4",
			input: `2025/03/15 10:47:59 [notice] 33709#33709: using inherited sockets from "5;6;"`,
			expectedOutput: protocol.Message{
				Data:      []byte(`using inherited sockets from "5;6;"`),
				Hostname:  localHostname,
				Timestamp: timeParse2Panic("2025/03/15 10:47:59"),
				Fields: map[string]any{
					iomodules.CFappname:   "-",
					iomodules.CFprocessid: 33709,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  "notice",
				},
			},
		},
		{
			name:  "Format Type 5",
			input: `2025-12-03 17:46:26 status triggers-pending libc-bin:amd64 2.41-12`,
			expectedOutput: protocol.Message{
				Data:      []byte(`status triggers-pending libc-bin:amd64 2.41-12`),
				Hostname:  localHostname,
				Timestamp: timeParse3Panic("2025-12-03 17:46:26"),
				Fields: map[string]any{
					iomodules.CFappname:   "-",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 6",
			input: `10.10.10.10 - - [28/Jul/2024:03:58:35 -0700] "GET / HTTP/1.1" 444 0 "-" "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0"`,
			expectedOutput: protocol.Message{
				Data:      []byte(`10.10.10.10 - - [28/Jul/2024:03:58:35 -0700] "GET / HTTP/1.1" 444 0 "-" "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0"`),
				Hostname:  localHostname,
				Timestamp: timeParse5Panic("28/Jul/2024:03:58:35 -0700"),
				Fields: map[string]any{
					iomodules.CFappname:   "-",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 7",
			input: `[19-Sep-2023 16:52:51] NOTICE: Terminating ...`,
			expectedOutput: protocol.Message{
				Data:      []byte(`Terminating ...`),
				Hostname:  localHostname,
				Timestamp: timeParse4Panic("19-Sep-2023 16:52:51"),
				Fields: map[string]any{
					iomodules.CFappname:   "-",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 8",
			input: `2025-12-21T18:39:01.211585-08:00 Host1 systemd[1]: Starting phpsessionclean.service - Clean php session files...`,
			expectedOutput: protocol.Message{
				Data:      []byte(`Starting phpsessionclean.service - Clean php session files...`),
				Hostname:  "Host1",
				Timestamp: timeParse6Panic("2025-12-21T18:39:01.211585-08:00"),
				Fields: map[string]any{
					iomodules.CFappname:   "systemd",
					iomodules.CFprocessid: 1,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
				},
			},
		},
		{
			name:  "Format Type 8 No pid",
			input: `2025-12-21T19:08:28.506905-08:00 Host1 php8.4-cgi: php_invoke mbstring: already enabled for PHP 8.4 cgi sapi`,
			expectedOutput: protocol.Message{
				Data:      []byte(`php_invoke mbstring: already enabled for PHP 8.4 cgi sapi`),
				Hostname:  "Host1",
				Timestamp: timeParse6Panic("2025-12-21T19:08:28.506905-08:00"),
				Fields: map[string]any{
					iomodules.CFappname:   "php8.4-cgi",
					iomodules.CFprocessid: testPid,
					iomodules.CFfacility:  iomodules.DefaultFacility,
					iomodules.CFseverity:  iomodules.DefaultSeverity,
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
			if !bytes.Equal(output.Data, tt.expectedOutput.Data) {
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

			expected := tt.expectedOutput.Fields[iomodules.CFappname]
			got, ok := output.Fields[iomodules.CFappname]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", iomodules.CFappname)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", iomodules.CFappname, expected, got)
			}

			expected = tt.expectedOutput.Fields[iomodules.CFfacility]
			got, ok = output.Fields[iomodules.CFfacility]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", iomodules.CFfacility)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", iomodules.CFfacility, expected, got)
			}

			expected = tt.expectedOutput.Fields[iomodules.CFprocessid]
			got, ok = output.Fields[iomodules.CFprocessid]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", iomodules.CFprocessid)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", iomodules.CFprocessid, expected, got)
			}

			expected = tt.expectedOutput.Fields[iomodules.CFseverity]
			got, ok = output.Fields[iomodules.CFseverity]
			if !ok {
				t.Errorf("expected %s to be present, but found nothing in custom fields", iomodules.CFseverity)
			}
			if expected != got {
				t.Errorf("expected %s to be '%s', but got '%s'", iomodules.CFseverity, expected, got)
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

func TestFormatAsText(t *testing.T) {
	ctx := context.Background()
	nowTime := time.Now()

	tests := []struct {
		name           string
		input          protocol.Payload
		expectedOutput string
	}{
		{
			name: "All fields present",
			input: protocol.Payload{
				RemoteIP:  netip.MustParseAddr("127.0.0.1"),
				Hostname:  "localhost",
				Timestamp: nowTime,
				CustomFields: map[string]any{
					iomodules.CFappname:   "app1",
					iomodules.CFseverity:  "info",
					iomodules.CFfacility:  "daemon",
					iomodules.CFprocessid: "123",
				},
				Data: []byte("test message"),
			},
			expectedOutput: nowTime.Format(time.RFC3339Nano) + " 127.0.0.1/localhost app1[123]: daemon: [info]: test message",
		},
		{
			name: "Missing hostname",
			input: protocol.Payload{
				RemoteIP:  netip.MustParseAddr("127.0.0.1"),
				Timestamp: nowTime,
				CustomFields: map[string]any{
					iomodules.CFappname:   "app1",
					iomodules.CFseverity:  "info",
					iomodules.CFfacility:  "daemon",
					iomodules.CFprocessid: "123",
				},
				Data: []byte("test message"),
			},
			expectedOutput: nowTime.Format(time.RFC3339Nano) + " 127.0.0.1 app1[123]: daemon: [info]: test message",
		},
		{
			name: "Missing ip",
			input: protocol.Payload{
				RemoteIP:  netip.Addr{},
				Timestamp: nowTime,
				Hostname:  "localhost",
				CustomFields: map[string]any{
					iomodules.CFappname:   "app1",
					iomodules.CFseverity:  "info",
					iomodules.CFfacility:  "daemon",
					iomodules.CFprocessid: "123",
				},
				Data: []byte("test message"),
			},
			expectedOutput: nowTime.Format(time.RFC3339Nano) + " localhost app1[123]: daemon: [info]: test message",
		},
		{
			name: "Single extra custom field",
			input: protocol.Payload{
				RemoteIP:  netip.Addr{},
				Timestamp: nowTime,
				Hostname:  "localhost",
				CustomFields: map[string]any{
					iomodules.CFappname:   "app1",
					iomodules.CFseverity:  "info",
					iomodules.CFfacility:  "daemon",
					iomodules.CFprocessid: "123",
					"type":                "extra",
				},
				Data: []byte("test message"),
			},
			expectedOutput: nowTime.Format(time.RFC3339Nano) + " localhost app1[123]: daemon: [info]: test message (type=extra)",
		},
		{
			name: "Multiple extra custom field",
			input: protocol.Payload{
				RemoteIP:  netip.Addr{},
				Timestamp: nowTime,
				Hostname:  "localhost",
				CustomFields: map[string]any{
					iomodules.CFappname:   "app1",
					iomodules.CFseverity:  "info",
					iomodules.CFfacility:  "daemon",
					iomodules.CFprocessid: "123",
					"type":                "extra",
					"field1":              "value1",
					"place":               "location",
				},
				Data: []byte("test message"),
			},
			expectedOutput: nowTime.Format(time.RFC3339Nano) + " localhost app1[123]: daemon: [info]: test message (field1=value1;place=location;type=extra)",
		},
		{
			name: "Unsupported custom field",
			input: protocol.Payload{
				RemoteIP:  netip.Addr{},
				Timestamp: nowTime,
				Hostname:  "localhost",
				CustomFields: map[string]any{
					iomodules.CFappname:   "app1",
					iomodules.CFseverity:  "info",
					iomodules.CFfacility:  "daemon",
					iomodules.CFprocessid: "123",
					"fieldinvalid":        struct{}{},
				},
				Data: []byte("test message"),
			},
			expectedOutput: nowTime.Format(time.RFC3339Nano) + " localhost app1[123]: daemon: [info]: test message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatAsText(ctx, &tt.input)
			if output != tt.expectedOutput {
				t.Fatalf("output mismatch:\nexpected:%q\ngot:     %q\n", tt.expectedOutput, output)
			}
		})
	}
}
