package syslog

import (
	"fmt"
	"sync"
)

// Initialize maps for both facility and severity
var facilityMu sync.RWMutex
var logFacility = LogFacility{
	FacilityToCode: map[string]uint16{
		"kern":     0,
		"user":     1,
		"mail":     2,
		"daemon":   3,
		"auth":     4,
		"syslog":   5,
		"lpr":      6,
		"news":     7,
		"uucp":     8,
		"cron":     9,
		"authpriv": 10,
		"ftp":      11,
		"local0":   16,
		"local1":   17,
		"local2":   18,
		"local3":   19,
		"local4":   20,
		"local5":   21,
		"local6":   22,
		"local7":   23,
	},
	CodeToFacility: make(map[uint16]string),
}
var severityMu sync.RWMutex
var logSeverity = LogSeverity{
	SeverityToCode: map[string]uint16{
		"emerg":   0,
		"alert":   1,
		"crit":    2,
		"err":     3,
		"warning": 4,
		"notice":  5,
		"info":    6,
		"debug":   7,
	},
	CodeToSeverity: make(map[uint16]string),
}

// Initialize reverse lookup maps
func InitBidiMaps() {
	facilityMu.Lock()
	defer facilityMu.Unlock()

	// Populate reverse lookup maps for facilities
	for facility, code := range logFacility.FacilityToCode {
		logFacility.CodeToFacility[code] = facility
	}

	severityMu.Lock()
	defer severityMu.Unlock()

	// Populate reverse lookup maps for severities
	for severity, code := range logSeverity.SeverityToCode {
		logSeverity.CodeToSeverity[code] = severity
	}
}

// Convert facility string to numeric code
func FacilityToCode(facility string) (code uint16, err error) {
	facilityMu.Lock()
	defer facilityMu.Unlock()

	code, exists := logFacility.FacilityToCode[facility]
	if !exists {
		err = fmt.Errorf("unknown facility name: %s", facility)
	}
	return
}

// Convert severity string to numeric code
func SeverityToCode(severity string) (code uint16, err error) {
	severityMu.Lock()
	defer severityMu.Unlock()

	code, exists := logSeverity.SeverityToCode[severity]
	if !exists {
		err = fmt.Errorf("unknown severity name: %s", severity)
	}
	return
}

// Convert facility code to string
func CodeToFacility(code uint16) (facility string, err error) {
	facilityMu.Lock()
	defer facilityMu.Unlock()

	facility, exists := logFacility.CodeToFacility[code]
	if !exists {
		err = fmt.Errorf("unknown facility code: %d", code)
	}
	return
}

// Convert severity code to string
func CodeToSeverity(code uint16) (severity string, err error) {
	severityMu.Lock()
	defer severityMu.Unlock()

	severity, exists := logSeverity.CodeToSeverity[code]
	if !exists {
		err = fmt.Errorf("unknown severity code: %d", code)
	}
	return
}
