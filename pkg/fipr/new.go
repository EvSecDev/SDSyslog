// Implementation of the Fragment Inter-Process Routing Protocol
package fipr

import (
	"fmt"
	"net"
)

// Creates a new fragment inter-process routing session.
// Session is created on top of an existing network connection.
// HMAC secret is used to ensure integrity of messages and to authenticate the remote end.
func New(conn net.Conn, hmacSecret []byte) (new *Session, err error) {
	if len(hmacSecret) != HMACSize {
		err = fmt.Errorf("%w: must be %d bytes long", ErrInvalidHMAC, HMACSize)
		return
	}
	if conn == nil {
		err = fmt.Errorf("%w: connection is nil", ErrTransportFailure)
		return
	}
	new = &Session{
		conn:       conn,
		state:      stateInit, // Prevents performing operations prior to receiving start
		seq:        0,         // Always starts at 0
		hmacSecret: hmacSecret,
		sentFrames: make(map[uint16]framebody),
	}
	return
}
