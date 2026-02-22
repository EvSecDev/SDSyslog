package fipr

import (
	"fmt"
)

// Fire and forget acknowledgement message of provided sequence number
func (session *Session) sendAck(seqToAck uint16) (err error) {
	_, err = session.send(opAck, encodeSeq(seqToAck))
	return
}

// Blocks until message is received. Error when received opcode is not ack.
// Ensures received acknowledgement message is for the provided expected sequence number.
func (session *Session) awaitAck(expectedAckdSeq uint16) (err error) {
	for range maxConsecutiveResends {
		var response *framebody
		response, err = session.await(opAck, opResend)
		if err != nil {
			session.Close()
			if err == ErrInvalidOpcode {
				err = ErrReceivedNoAck
			}
			err = fmt.Errorf("await: %w", err)
			return
		}

		if response.op == opResend {
			seqToResend := decodeSeq(response.payload)
			var sentSequence uint16
			sentSequence, err = session.resend(seqToResend)
			if err != nil {
				err = fmt.Errorf("resend: %w", err)
				return
			}

			// New resent frame is already sent over the wire and is recorded under sentSequence.
			// Remove the old seq from the record or it will be dangling.
			session.resendMutex.Lock()
			delete(session.sentFrames, expectedAckdSeq)
			session.resendMutex.Unlock()

			// Go back to waiting for ack for our resent payload
			expectedAckdSeq = sentSequence
			continue
		}

		// Ensure received acknowledgement was for the frame we wanted an ack for
		ackdSeq := decodeSeq(response.payload)
		if ackdSeq != expectedAckdSeq {
			session.Close()
			err = ErrBadSequence
		} else {
			session.resendMutex.Lock()
			delete(session.sentFrames, expectedAckdSeq)
			session.consecResends = 0
			session.resendMutex.Unlock()
		}
		break
	}
	return
}

// Sends message with custom op code and payload and blocks until an acknowledgement
func (session *Session) send(op opCode, payload []byte) (sentSeq uint16, err error) {
	session.stateMutex.RLock()
	reqSeq := session.seq
	session.stateMutex.RUnlock()

	request, err := session.encodeFrame(op, payload)
	if err != nil {
		err = fmt.Errorf("encode: %w", err)
		return
	}
	err = session.writeFrame(request)

	if requiresAck(op) {
		// Success if (real) frame was acknowledged
		err = session.awaitAck(reqSeq)
		if err != nil {
			err = fmt.Errorf("await acknowledge: %w", err)
			return
		}
	}

	sentSeq = reqSeq
	return
}

// Blocks until message is received and acknowledgement is sent. Error when received opcode is not expected opcode.
// Success if any single expectedOp is received.
func (session *Session) await(expectedOps ...opCode) (responseFrame *framebody, err error) {
	var prevSeqForResendReq uint16

	for range maxConsecutiveResends {
		// Await response
		var wireFrame []byte
		wireFrame, err = session.readFrame()
		if err != nil {
			err = fmt.Errorf("frame read: %w", err)
			return
		}

		// Parse response
		responseFrame, err = session.decodeFrame(wireFrame)
		if isRetryableError(err) {
			if len(wireFrame) < lenFieldFrameLen+lenFieldSequence {
				err = fmt.Errorf("%w: original error: %w", ErrResendWithoutSeq, err)
				return
			}

			// Request resend for this sequence
			seqToResend := wireFrame[lenFieldFrameLen : lenFieldFrameLen+lenFieldSequence]

			var sentSeq uint16
			sentSeq, err = session.send(opResend, seqToResend)
			if err != nil {
				err = fmt.Errorf("send: %w", err)
				return
			}
			session.resendMutex.Lock()
			session.consecResends++
			session.resendMutex.Unlock()

			// New frame already added to tracker, delete the old one.
			if prevSeqForResendReq > 0 {
				session.resendMutex.Lock()
				delete(session.sentFrames, prevSeqForResendReq)
				session.resendMutex.Unlock()
			}

			// Go back to waiting for the original opcode
			prevSeqForResendReq = sentSeq
			continue
		} else if err != nil {
			// Fatal transport or protocol error
			session.Close()
			err = fmt.Errorf("decode: %w", err)
			return
		}

		// Regardless of actual opcode match, no more resends from here on
		session.resendMutex.Lock()
		session.consecResends = 0
		session.resendMutex.Unlock()

		// Success on first match
		var opCodeMatches bool
		for _, expectedOp := range expectedOps {
			if responseFrame.op == expectedOp {
				opCodeMatches = true
				break
			}
		}
		if opCodeMatches && requiresAck(responseFrame.op) {
			// Send acknowledgement for regular requests
			err = session.sendAck(responseFrame.sequence)
			if err != nil {
				err = fmt.Errorf("send acknowledge: %w", err)
				return
			}
		} else if !opCodeMatches {
			session.Close()
			err = ErrInvalidOpcode
		}

		return
	}

	session.Close()
	err = ErrTooManyResends
	return
}

// Resends a sequence ID. Does not await new ack (caller must await)
func (session *Session) resend(seqToResend uint16) (sentSeq uint16, err error) {
	if session.consecResends > maxConsecutiveResends {
		session.Close()
		err = ErrTooManyResends
		return
	}

	session.resendMutex.Lock()
	oldFrame, ok := session.sentFrames[seqToResend]
	session.resendMutex.Unlock()

	if !ok {
		session.Close()
		err = ErrInvalidResend
		return
	}

	session.resendMutex.Lock()
	session.consecResends++
	session.resendMutex.Unlock()

	session.stateMutex.RLock()
	sentSeq = session.seq
	session.stateMutex.RUnlock()

	// Fire and forget
	resendFrame, err := session.encodeFrame(oldFrame.op, oldFrame.payload)
	if err != nil {
		err = fmt.Errorf("encode: %w", err)
		return
	}
	err = session.writeFrame(resendFrame)
	return
}
