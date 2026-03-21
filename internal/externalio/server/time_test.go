package server

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseTimeRange(t *testing.T) {
	testCurrentTime := time.Now()

	tests := []struct {
		name         string
		rawStartTime string
		rawEndTime   string
		expectStart  string
		expectEnd    string
		expectError  error
	}{
		{
			name:         "basic absolute",
			rawStartTime: "2006-01-06T06:32:12Z",
			rawEndTime:   "2006-01-06T06:32:13Z",
			expectStart:  "2006-01-06 06:32:12 +0000 UTC",
			expectEnd:    "2006-01-06 06:32:13 +0000 UTC",
		},
		{
			name:         "basic relative",
			rawStartTime: "-5m",
			rawEndTime:   "-1m",
			expectStart:  testCurrentTime.Add(-5 * time.Minute).String(),
			expectEnd:    testCurrentTime.Add(-1 * time.Minute).String(),
		},
		{
			name:         "basic absolute and relative",
			rawStartTime: "2020-01-08T06:32:13Z",
			rawEndTime:   "-30m",
			expectStart:  "2020-01-08 06:32:13 +0000 UTC",
			expectEnd:    testCurrentTime.Add(-30 * time.Minute).String(),
		},
		{
			name:         "invalid start relative",
			rawStartTime: "-5w",
			rawEndTime:   "-30m",
			expectError:  fmt.Errorf("invalid relative end time \"-30m\""),
		},
		{
			name:         "invalid start absolute",
			rawStartTime: "2026-04-03",
			rawEndTime:   "-30m",
			expectError:  fmt.Errorf("invalid start time \"2026-04-03\""),
		},
		{
			name:         "invalid end relative",
			rawStartTime: "-5m",
			rawEndTime:   "-3w",
			expectError:  fmt.Errorf("invalid relative end time \"-3w\""),
		},
		{
			name:         "invalid end absolute",
			rawStartTime: "-30m",
			rawEndTime:   "2022-01-12",
			expectError:  fmt.Errorf("invalid end time \"2022-01-12\""),
		},
		{
			name:         "start after end",
			rawStartTime: "-30m",
			rawEndTime:   "-60m",
			expectError:  fmt.Errorf("start time cannot be after end time"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parseTimeRange(testCurrentTime, tt.rawStartTime, tt.rawEndTime)
			if err != nil && tt.expectError == nil {
				t.Fatalf("unexpected parsing error: '%v'", err)
			}
			if err == nil && tt.expectError != nil {
				t.Fatalf("expected error '%v', but got nil", tt.expectError)
			}
			if err != nil && !strings.HasPrefix(err.Error(), tt.expectError.Error()) {
				t.Fatalf("expected error '%v', but got error '%v'", tt.expectError, err)
			}
			if err != nil && strings.HasPrefix(err.Error(), tt.expectError.Error()) {
				return
			}

			if start.String() != tt.expectStart {
				t.Errorf("expected parsed start time %q, but got %q", tt.expectStart, start.String())
			}
			if end.String() != tt.expectEnd {
				t.Errorf("expected parsed end time %q, but got %q", tt.expectEnd, end.String())
			}
		})
	}
}
