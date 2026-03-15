package protocol

import (
	"net/netip"
	"sdsyslog/internal/filtering"
	"time"
)

// Container for external use - mandatory parts
type Message struct {
	Timestamp time.Time
	Hostname  string
	Fields    map[string]any
	Data      []byte
}

// Boolean filter config for a message
type MessageFilter struct {
	// Filters for top-level string fields
	Data *filtering.Filter `json:"data,omitempty"`

	// Filters for map keys/values (match any key or value)
	FieldsKey   *filtering.Filter `json:"fieldsKey,omitempty"`
	FieldsValue *filtering.Filter `json:"fieldsValue,omitempty"`

	// Optional: match only if all fields match (AND) or any field matches (OR)
	UseAnd bool `json:"use_and,omitempty"` // default true = AND
}

// Friendly container for internal use - most fields are optional (filled in automatically) - leave exported
type Payload struct {
	RemoteIP      netip.Addr // Derived by receiver, not sent across the wire
	HostID        int
	MsgID         int
	MessageSeq    int
	MessageSeqMax int
	Timestamp     time.Time
	Hostname      string
	SignatureID   uint8
	Signature     []byte
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
	SignatureID   uint8
	Signature     []byte
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
