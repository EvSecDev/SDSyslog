package protocol

import "time"

type LogFacility struct {
	FacilityToCode map[string]uint16
	CodeToFacility map[uint16]string
}

type LogSeverity struct {
	SeverityToCode map[string]uint16
	CodeToSeverity map[uint16]string
}

// Container for external use - fields are mandatory
type Message struct {
	Facility        string
	Severity        string
	Timestamp       time.Time
	ProcessID       int
	Hostname        string
	ApplicationName string
	LogText         string
}

// Friendly container for internal use - most fields are optional (filled in automatically) - leave exported
type Payload struct {
	RemoteIP        string // Derived by receiver, not sent across the wire
	HostID          int
	LogID           int
	MessageSeq      int
	MessageSeqMax   int
	Facility        string
	Severity        string
	Timestamp       time.Time
	ProcessID       int
	Hostname        string
	ApplicationName string
	LogText         []byte
	PaddingLen      int
}

// Container for post-payload-serialization
type InnerWireFormat struct {
	HostID          uint32
	LogID           uint32
	MessageSeq      uint16
	MessageSeqMax   uint16
	Facility        uint16
	Severity        uint16
	Timestamp       uint64
	ProcessID       uint32
	Hostname        []byte
	ApplicationName []byte
	LogText         []byte
	PaddingLen      int
}
