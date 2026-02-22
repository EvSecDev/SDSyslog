package fipr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

// Writes frame to transport layer connection.
func (session *Session) writeFrame(wireFrame []byte) (err error) {
	// Safety only
	err = session.conn.SetReadDeadline(time.Now().Add(maxWaitTimeForSend))
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			session.Close()
		} else {
			err = fmt.Errorf("failed setting write deadline: %w", err)
		}
		return
	}
	defer session.conn.SetReadDeadline(time.Time{})

	for len(wireFrame) > 0 {
		var n int
		n, err = session.conn.Write(wireFrame)
		if err != nil {
			err = fmt.Errorf("write: %w: %w", ErrTransportFailure, err)
			return
		}
		wireFrame = wireFrame[n:]
	}
	return
}

// Blocks until a single full frame is available.
func (session *Session) readFrame() (wireFrame []byte, err error) {
	// Safety only
	err = session.conn.SetReadDeadline(time.Now().Add(maxWaitTimeForFrame))
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			session.Close()
		} else {
			err = fmt.Errorf("failed setting read deadline: %w", err)
		}
		return
	}
	defer session.conn.SetReadDeadline(time.Time{})

	for {
		// Attempt to extract frame from internal buffer
		var n int
		n, wireFrame, err = tryParseFrame(session.transportBuffer)
		if err != nil {
			return
		} else if len(wireFrame) > 0 {
			session.transportBuffer = session.transportBuffer[n:] // keep leftovers
			return
		}

		// Otherwise read additional bytes from OS buffer to internal buffer
		tmp := make([]byte, 4096)
		n, err = session.conn.Read(tmp)
		if err != nil {
			err = fmt.Errorf("read: %w: %w", ErrTransportFailure, err)
			return
		}
		session.transportBuffer = append(session.transportBuffer, tmp[:n]...)
	}
}

// Attempts to extract a full frame from buffer.
// Only returns errors when frame length field is below protocol minimum
func tryParseFrame(buf []byte) (consumed int, frame []byte, err error) {
	// Need at least the length field
	if len(buf) < lenFieldFrameLen {
		return
	}

	// Read declared frame length
	frameLen := binary.BigEndian.Uint32(buf[:4])

	// Sanity checks
	if frameLen < uint32(minFrameLen) {
		// Unrecoverable as a session is serial
		err = ErrInvalidFrameLen
		return
	}

	totalLen := lenFieldFrameLen + int(frameLen)

	// Not enough data yet
	if len(buf) < totalLen {
		return
	}

	// We have a full frame
	frame = buf[:totalLen]
	consumed = totalLen
	return
}
