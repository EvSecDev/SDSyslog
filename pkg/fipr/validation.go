package fipr

// Processes a decoded frame to check is validity.
// Any fatal errors are logged and the session will be closed.
func (session *Session) validateFrame(frame *framebody) (err error) {
	if session.IsClosed() {
		err = ErrSessionClosed
		return
	}

	switch frame.op {
	case opStart:
		session.stateMutex.RLock()
		currentSessionState := session.state
		session.stateMutex.RUnlock()

		if currentSessionState != stateInit {
			session.Close()
			err = ErrLateStart
		} else if frame.sequence != 0 {
			session.Close()
			err = ErrOutOfOrderStart
		}
		if len(frame.payload) <= 0 {
			err = ErrFrameHasNoPayload
		}
	case opOBO:
		if len(frame.payload) == 0 {
			err = ErrFrameHasNoPayload
		}
	case opShardCheck, opMsgCheck:
		if len(frame.payload) > 0 {
			err = ErrFrameHasPayload
		}
	case opShardStatus, opMsgStatus:
		if len(frame.payload) == 0 {
			err = ErrFrameHasNoPayload
		}
		if len(frame.payload) > 1 {
			err = ErrPayloadWrongLength
		}
	case opFrgRoute:
		if len(frame.payload) == 0 {
			err = ErrFrameHasNoPayload
		}
	case opResend, opAck:
		if len(frame.payload) != 2 {
			err = ErrPayloadWrongLength
		}
	case opAccepted:
		session.Close()
	case opRejected:
		err = ErrRemoteRejected
	default:
		session.Close()
		err = ErrInvalidOpcode
	}
	return
}
