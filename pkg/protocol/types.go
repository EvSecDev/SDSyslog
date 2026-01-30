package protocol

import "time"

// Container for external use - fields are mandatory
type Message struct {
	Timestamp time.Time
	Hostname  string
	Fields    map[string]any
	Data      string
}

// Friendly container for internal use - most fields are optional (filled in automatically) - leave exported
type Payload struct {
	RemoteIP      string // Derived by receiver, not sent across the wire
	HostID        int
	MsgID         int
	MessageSeq    int
	MessageSeqMax int
	Timestamp     time.Time
	Hostname      string
	CustomFields  map[string]any
	Data          []byte
	PaddingLen    int
}

// Container for post-payload-validation
type innerWireFormat struct {
	HostID        uint32
	MsgID         uint32
	MessageSeq    uint16
	MessageSeqMax uint16
	Timestamp     uint64
	Hostname      []byte
	ContextFields []contextWireFormat
	Data          []byte
	PaddingLen    int
}

// Container for post-payload-validation
type contextWireFormat struct {
	Key     []byte
	valType uint8
	Value   []byte
}
