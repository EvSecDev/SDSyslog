package fipr

import "errors"

// Reports whether the protocol is closed
func (session *Session) IsClosed() (closed bool) {
	session.stateMutex.RLock()
	defer session.stateMutex.RUnlock()
	closed = session.state == stateClosed
	return
}

// Shuts down session. Does NOT shutdown underlying connection.
func (session *Session) Close() (isClosed bool) {
	session.stateMutex.Lock()
	defer session.stateMutex.Unlock()
	session.state = stateClosed
	return
}

// Set the original (on behalf of) sender address for the session (only allowed once)
func (session *Session) setOriginalSender(originalSenderAddress []byte) {
	session.oboOnce.Do(func() {
		session.obo = originalSenderAddress
	})
}

// Retrieves the on-behalf-of (original sender) address for this connection
func (session *Session) OriginalSender() (originalSenderAddress string) {
	originalSenderAddress = string(session.obo)
	return
}

// Set the session message id (only allowed once)
func (session *Session) setMessageID(id []byte) {
	session.messageIDOnce.Do(func() {
		session.messageID = id
	})
}

// Retrieves the on-behalf-of (original sender) address for this connection
func (session *Session) MessageID() (id string) {
	id = string(session.messageID)
	return
}

// Checks if opcode requires acknowledgement
func requiresAck(op opCode) (ack bool) {
	switch op {
	case opAck:
		ack = false
	case opResend:
		ack = false
	case opAccepted:
		ack = false
	case opRejected:
		ack = false
	default:
		ack = true
	}
	return
}

// Contains list of errors that can cause a resend request.
func isRetryableError(err error) (retry bool) {
	if err == nil {
		return
	}

	for _, target := range []error{
		ErrFrameHasNoPayload,
		ErrFrameHasPayload,
		ErrPayloadWrongLength,
	} {
		if errors.Is(err, target) {
			retry = true
			return
		}
	}
	return
}
