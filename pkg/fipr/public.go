package fipr

import (
	"fmt"
)

// Begins a new session with given message identifier.
func (session *Session) SendStart(messageID string) (err error) {
	if messageID == "" {
		err = fmt.Errorf("%w: message ID cannot be empty", ErrFrameHasNoPayload)
		return
	}

	_, err = session.send(opStart, []byte(messageID))
	if err != nil {
		err = fmt.Errorf("send start: %w", err)
		return
	}

	session.stateMutex.Lock()
	session.state = stateStarted
	session.stateMutex.Unlock()
	return
}

// Waits until client starts session.
func (session *Session) WaitStart() (err error) {
	recvFrame, err := session.await(opStart)
	if err != nil {
		err = fmt.Errorf("await start: %w", err)
		return
	}
	if len(recvFrame.payload) == 0 {
		err = ErrFrameHasNoPayload
		return
	}

	session.setMessageID(recvFrame.payload)

	session.stateMutex.Lock()
	session.state = stateStarted
	session.stateMutex.Unlock()
	return
}

// Tells client fragment was accepted and closes session.
func (session *Session) SendAccept() (err error) {
	_, err = session.send(opAccepted, nil)
	if err != nil {
		err = fmt.Errorf("send accept: %w", err)
		return
	}
	session.Close()
	return
}

// Tells client their request/session/fragment was rejected
func (session *Session) SendReject() (err error) {
	_, err = session.send(opRejected, nil)
	if err != nil {
		err = fmt.Errorf("send reject: %w", err)
		return
	}
	session.Close()
	return
}

// Tells server which original sender this session is for.
func (session *Session) SendOnBehalfOf(originalSender string) (err error) {
	_, err = session.send(opOBO, []byte(originalSender))
	if err != nil {
		err = fmt.Errorf("send onbehalfof: %w", err)
	}
	return
}

// Waits for the original sender from the client
func (session *Session) WaitOnBehalfOf() (err error) {
	recvFrame, err := session.await(opOBO)
	if err != nil {
		err = fmt.Errorf("await onbehalfof: %w", err)
		return
	}
	session.setOriginalSender(recvFrame.payload)
	return
}

// Requests server shard current status.
// If not draining and no error, shard is fully running.
func (session *Session) SendShardCheck() (draining bool, err error) {
	// Send shard check
	_, err = session.send(opShardCheck, nil)
	if err != nil {
		err = fmt.Errorf("send shard check: %w", err)
		return
	}

	// Await shard response
	recvFrame, err := session.await(opShardStatus)
	if err != nil {
		err = fmt.Errorf("await shard status: %w", err)
		return
	}

	currentStatus := shardStatus(recvFrame.payload[0])
	switch currentStatus {
	case shardDraining:
		draining = true
	case shardShutdown:
		err = ErrShardShutdown
	}
	return
}

// Waits for the shard check request from the client.
func (session *Session) WaitShardCheck() (err error) {
	_, err = session.await(opShardCheck)
	if err != nil {
		err = fmt.Errorf("await shard check: %w", err)
	}
	return
}

// Sends current shard status to client
func (session *Session) SendShardStatus(running bool, draining bool) (err error) {
	var body byte
	if running && !draining {
		body = byte(shardRunning)
	} else if draining {
		body = byte(shardDraining)
	} else {
		body = byte(shardShutdown)
	}

	_, err = session.send(opShardStatus, []byte{body})
	if err != nil {
		err = fmt.Errorf("send shard status: %w", err)
		return
	}
	return
}

// Requests message identifiers current status.
// Reports whether message partially exists on remote shard or is unseen.
func (session *Session) SendMessageCheck() (msgExists bool, err error) {
	// Send message check
	_, err = session.send(opMsgCheck, nil)
	if err != nil {
		err = fmt.Errorf("send message check: %w", err)
		return
	}

	// Await message response
	recvFrame, err := session.await(opMsgStatus)
	if err != nil {
		err = fmt.Errorf("await message status: %w", err)
		return
	}

	currentStatus := messageStatus(recvFrame.payload[0])
	if currentStatus == msgExisting {
		msgExists = true
	}
	// msgNew implies message has not been seen by server
	return
}

// Waits for the message check request from the client.
func (session *Session) WaitMessageCheck() (err error) {
	_, err = session.await(opMsgCheck)
	if err != nil {
		err = fmt.Errorf("await message check: %w", err)
	}
	return
}

// Sends current message status to client
func (session *Session) SendMessageStatus(exists bool) (err error) {
	var body byte
	if exists {
		body = byte(msgExisting)
	} else {
		body = byte(msgNew)
	}

	_, err = session.send(opMsgStatus, []byte{body})
	return
}

// Send the fragment to the server
func (session *Session) SendFragment(fragment []byte) (err error) {
	_, err = session.send(opFrgRoute, fragment)
	if err != nil {
		err = fmt.Errorf("fragment send failed: %w", err)
		return
	}

	_, err = session.await(opAccepted)
	if err != nil {
		if err == ErrInvalidOpcode {
			err = ErrRemoteRejected
		}
		err = fmt.Errorf("did not receive accepted response to fragment: %w", err)
		return
	}
	return
}

// Wait for client to send fragment.
func (session *Session) WaitFragment() (fragment []byte, err error) {
	recvFrame, err := session.await(opFrgRoute)
	if err != nil {
		err = fmt.Errorf("await fragment: %w", err)
		return
	}
	fragment = recvFrame.payload
	return
}
