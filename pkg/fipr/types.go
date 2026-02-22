package fipr

import (
	"net"
	"sync"
)

// Manages protocol session state and frame processing
type Session struct {
	// Connection identities
	messageID     []byte    // Primary connection identifier
	messageIDOnce sync.Once // Only permit setting once
	obo           []byte    // original fragment sender address
	oboOnce       sync.Once // Only permit setting once
	hmacSecret    []byte    // Used for signing outbound messages and verifying inbound messages

	// Session state
	stateMutex sync.RWMutex // Synchronize access to transport, session, and sequence
	conn       net.Conn     // Underlying transport connection (Unix domain socket)
	state      sessionState // Represents if the session is pre-start, accepting messages, or shutdown
	seq        uint16       // Session-only counter - ALWAYS incrementing (sender, receiver, acks, resends, ect.)

	// Framing
	transportBuffer []byte // Buffer raw bytes from transport connection

	// Retransmission
	resendMutex   sync.Mutex
	sentFrames    map[uint16]framebody // Tracking all session payloads for resend requests
	consecResends int
}

// Decoded protocol frame body (not including len field or hmac field)
type framebody struct {
	sequence uint16
	op       opCode
	payload  []byte
}

type (
	sessionState  int
	messageStatus byte
	shardStatus   byte
	opCode        byte
)
