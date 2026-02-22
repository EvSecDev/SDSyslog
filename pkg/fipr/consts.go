package fipr

import (
	"errors"
	"time"
)

const (
	HMACSize         int = lenFieldHMAC
	lenFieldFrameLen int = 4
	lenFieldSequence int = 2
	lenFieldOpCode   int = 1
	lenFieldHMAC     int = 16
	maxDataLength    int = 65535
	minDataLength    int = lenFieldFrameLen + lenFieldSequence + lenFieldOpCode + lenFieldHMAC
	minFrameLen      int = lenFieldSequence + lenFieldOpCode + lenFieldHMAC

	// Protocol Codes
	opStart       opCode = 0x00
	opOBO         opCode = 0x05
	opAck         opCode = 0x10
	opResend      opCode = 0x11
	opAccepted    opCode = 0x12
	opRejected    opCode = 0x13
	opShardCheck  opCode = 0x20
	opShardStatus opCode = 0x21
	opMsgCheck    opCode = 0x22
	opMsgStatus   opCode = 0x23
	opFrgRoute    opCode = 0x24

	shardRunning  shardStatus = 0x01
	shardDraining shardStatus = 0x11
	shardShutdown shardStatus = 0x22

	msgNew      messageStatus = 0x00
	msgExisting messageStatus = 0x10

	// Internal state use
	stateInit    sessionState = 0
	stateStarted sessionState = 1
	stateClosed  sessionState = 2

	maxConsecutiveResends int    = 3
	maxSequence           uint16 = (1 << (8 * lenFieldSequence)) - 1
	// Strict deadlines as this is only supposed to be implemented over local os sockets
	maxWaitTimeForFrame time.Duration = 3 * time.Second // Maximum time any one read can spend waiting for input
	maxWaitTimeForSend  time.Duration = 2 * time.Second // Maximum time any one write can spend writing to the transport connection
)

var (
	ErrTransportFailure   = errors.New("transport layer encountered an error")
	ErrFrameHasPayload    = errors.New("frame has opcode that requires empty payload but has a non-empty payload")
	ErrFrameHasNoPayload  = errors.New("frame has opcode that requires non-empty payload but has an empty payload")
	ErrInvalidFrameLen    = errors.New("frame from transport layer has invalid length field")
	ErrPayloadWrongLength = errors.New("payload for opcode is not the correct length")
	ErrFrameTooShort      = errors.New("frame too short")
	ErrFrameTooLarge      = errors.New("frame too large")
	ErrOutOfOrderStart    = errors.New("received start opcode but sequence is not 0")
	ErrLateStart          = errors.New("received start opcode but session had already started or shutdown")
	ErrUnexpectedResponse = errors.New("received unexpected opcode in response")
	ErrBadSequence        = errors.New("unexpected sequence")
	ErrWrappedSequence    = errors.New("sequence number hit maximum")
	ErrInvalidResend      = errors.New("resend requested sequence that did not happen")
	ErrResendWithoutSeq   = errors.New("unable to recover from retryable error due to too short of a frame")
	ErrTooManyResends     = errors.New("exceeded maximum resend threshold")
	ErrInvalidHMAC        = errors.New("invalid hmac")
	ErrInvalidOpcode      = errors.New("invalid opcode for state")
	ErrReceivedNoAck      = errors.New("did not receive acknowledgement")
	ErrRemoteRejected     = errors.New("remote end sent rejected op code")
	ErrShardShutdown      = errors.New("server cannot accept any fragments as it is shutdown")
	ErrSessionClosed      = errors.New("session closed")
	ErrSessionShutdown    = errors.New("session gracefully terminated")
)
