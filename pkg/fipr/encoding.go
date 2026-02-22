package fipr

import (
	"encoding/binary"
	"fmt"
	"sdsyslog/internal/crypto/hmac"
)

// Creates byte slice from sequence unsigned integer
func encodeSeq(seq uint16) (sequence []byte) {
	sequence = make([]byte, lenFieldSequence)
	binary.BigEndian.PutUint16(sequence, seq)
	return
}

// Decodes byte slice to sequence unsigned integer
func decodeSeq(sequence []byte) (seq uint16) {
	seq = binary.BigEndian.Uint16(sequence)
	return
}

// Builds a wire frame from components and records it for resend.
func (session *Session) encodeFrame(op opCode, payload []byte) (wireFrame []byte, err error) {
	if session.IsClosed() {
		err = ErrSessionClosed
		return
	}

	session.stateMutex.Lock() // Hold lock for entire sequence safety check
	seq := session.seq
	if seq == maxSequence {
		session.state = stateClosed
		err = ErrBadSequence
		session.stateMutex.Unlock()
		return
	}
	session.stateMutex.Unlock()

	bodyLen := lenFieldSequence + lenFieldOpCode + len(payload) + HMACSize
	wireFrame = make([]byte, lenFieldFrameLen+bodyLen)

	// Field: Frame Length
	binary.BigEndian.PutUint32(wireFrame[0:lenFieldFrameLen], uint32(bodyLen))

	// Field: Sequence
	seqStart := lenFieldFrameLen
	copy(wireFrame[seqStart:seqStart+lenFieldSequence], encodeSeq(seq))

	// Field: opCode
	opCodeIndex := seqStart + lenFieldSequence
	wireFrame[opCodeIndex] = byte(op)

	// Field: Payload
	payloadStart := opCodeIndex + lenFieldOpCode
	copy(wireFrame[payloadStart:], payload)

	// Field: HMAC
	mac := hmac.ComputeSHA256(session.hmacSecret, HMACSize, wireFrame[:payloadStart+len(payload)])
	copy(wireFrame[payloadStart+len(payload):], mac)

	frame := framebody{
		sequence: seq,
		op:       op,
		payload:  payload,
	}

	session.stateMutex.Lock()
	session.seq++
	session.stateMutex.Unlock()

	session.resendMutex.Lock()
	session.sentFrames[seq] = frame
	session.resendMutex.Unlock()
	return
}

// Decodes and validates a single frame from raw bytes.
func (session *Session) decodeFrame(wireFrame []byte) (frame *framebody, err error) {
	if len(wireFrame) < minDataLength {
		err = ErrFrameTooShort
		return
	}
	if len(wireFrame) > maxDataLength {
		err = ErrFrameTooLarge
		return
	}

	// Field: Frame Length
	bodyLen := int(binary.BigEndian.Uint32(wireFrame[0:lenFieldFrameLen]))
	if bodyLen < minFrameLen {
		err = ErrInvalidFrameLen
		return
	}

	// Field: Sequence
	seqStart := lenFieldFrameLen
	seq := decodeSeq(wireFrame[seqStart : seqStart+lenFieldSequence])

	session.stateMutex.RLock()
	if seq != session.seq {
		err = fmt.Errorf("%w: received sequence %d but expected %d", ErrBadSequence, seq, session.seq)
		session.stateMutex.RUnlock()
		return
	}
	session.stateMutex.RUnlock()

	// Field: opCode
	opCodeIndex := lenFieldFrameLen + lenFieldSequence
	op := opCode(wireFrame[opCodeIndex])

	// Field: Payload
	payloadStart := opCodeIndex + lenFieldOpCode
	payloadLen := bodyLen - (lenFieldSequence + lenFieldOpCode + HMACSize)
	payloadEnd := payloadStart + payloadLen
	payload := wireFrame[payloadStart:payloadEnd]

	// Field: HMAC
	mac := wireFrame[payloadEnd:]
	valid := hmac.VerifySHA256(session.hmacSecret, HMACSize, wireFrame[:payloadEnd], mac)
	if !valid {
		// Immediately close
		session.state = stateClosed
		err = ErrInvalidHMAC
		return
	}

	rawFrame := &framebody{
		sequence: seq,
		op:       op,
		payload:  payload,
	}

	validationError := session.validateFrame(rawFrame)
	if validationError == nil || isRetryableError(validationError) {
		// Update tracked sequence number only for non-errors and retry errors
		session.stateMutex.Lock()
		session.seq++
		session.stateMutex.Unlock()
	}
	if validationError != nil {
		err = fmt.Errorf("validation: %w", validationError)
		return
	}

	frame = rawFrame
	return
}
