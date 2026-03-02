package fipr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

// Writes frame to transport layer connection.
func (session *Session) writeFrame(wireFrame []byte) (err error) {
	// Safety only
	err = session.conn.SetWriteDeadline(time.Now().Add(maxWaitTimeForSend))
	if err != nil {
		if transportWasClosed(err) {
			err = ErrTransportWasClosed
			session.Close()
		} else {
			err = fmt.Errorf("failed setting write deadline: %w", err)
		}
		return
	}

	defer func() {
		if err == nil { // Only reset deadline if no error occurred
			err = session.conn.SetWriteDeadline(time.Time{})
			if err != nil {
				err = fmt.Errorf("failed resetting write deadline: %w", err)
			}
		}
	}()

	for len(wireFrame) > 0 {
		var n int
		n, err = session.conn.Write(wireFrame)
		if err != nil {
			if transportWasClosed(err) {
				err = ErrTransportWasClosed
				session.Close()
			} else {
				err = fmt.Errorf("write: %w: %w", ErrTransportFailure, err)
			}
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
		if transportWasClosed(err) {
			err = ErrTransportWasClosed
			session.Close()
		} else {
			err = fmt.Errorf("failed setting read deadline: %w", err)
		}
		return
	}

	defer func() {
		if err == nil { // Only reset deadline if no error occurred
			err = session.conn.SetReadDeadline(time.Time{})
			if err != nil &&
				!errors.Is(err, os.ErrClosed) &&
				!errors.Is(err, net.ErrClosed) &&
				!errors.Is(err, io.ErrClosedPipe) {
				err = fmt.Errorf("failed resetting read deadline: %w", err)
			}
		}
	}()

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
			if transportWasClosed(err) {
				err = ErrTransportWasClosed
				session.Close()
			} else {
				err = fmt.Errorf("read: %w: %w", ErrTransportFailure, err)
			}
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

// Checks if supplied error is an error encountered when transport layer connection is closed
func transportWasClosed(err error) (closed bool) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, io.EOF):
		closed = true
	case errors.Is(err, io.ErrClosedPipe):
		closed = true
	case errors.Is(err, net.ErrClosed):
		closed = true
	}
	return
}
