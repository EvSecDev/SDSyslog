package server

import (
	"fmt"
	"time"
)

// Parses raw HTTP request start and end time semantic strings into timestamps
func parseTimeRangeNow(rawStartTime, rawEndTime string) (startTime, endTime time.Time, err error) {
	return parseTimeRange(time.Now(), rawStartTime, rawEndTime)
}

// Parses raw HTTP request start and end time semantic strings using supplied current time into timestamps
func parseTimeRange(currentTime time.Time, rawStartTime, rawEndTime string) (startTime, endTime time.Time, err error) {
	// Start Time
	if rawStartTime == "" {
		// Default start is last minute
		startTime = currentTime.Add(-1 * time.Minute)
	} else if rawStartTime[0] == '-' || rawStartTime[0] == '+' {
		var dur time.Duration
		dur, err = time.ParseDuration(rawStartTime)
		if err != nil {
			err = fmt.Errorf("invalid relative end time %q: %w", rawEndTime, err)
			return
		}
		startTime = currentTime.Add(dur)
	} else {
		startTime, err = time.Parse(time.RFC3339Nano, rawStartTime)
		if err != nil {
			err = fmt.Errorf("invalid start time %q: %w", rawStartTime, err)
			return
		}
	}

	// End Time
	if rawEndTime == "now" || rawEndTime == "" {
		// Default end is now
		endTime = currentTime
	} else if rawEndTime[0] == '-' || rawEndTime[0] == '+' {
		var dur time.Duration
		dur, err = time.ParseDuration(rawEndTime)
		if err != nil {
			err = fmt.Errorf("invalid relative end time %q: %w", rawEndTime, err)
			return
		}
		endTime = currentTime.Add(dur)
	} else {
		endTime, err = time.Parse(time.RFC3339Nano, rawEndTime)
		if err != nil {
			err = fmt.Errorf("invalid end time %q: %w", rawEndTime, err)
			return
		}
	}

	if startTime.After(endTime) {
		err = fmt.Errorf("start time cannot be after end time")
		return
	}
	return
}
