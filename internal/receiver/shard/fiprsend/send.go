package fiprsend

import (
	"fmt"
	"net"
	"net/netip"
	"runtime/debug"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/pkg/fipr"
	"sdsyslog/pkg/protocol"
)

// Sends fragment to another process.
// Any error should only be logged and fragment should route local.
func RouteFragment(socketPath string, messageID string, remoteAddress netip.Addr, fragment protocol.Payload) (rerouteLocally bool, err error) {
	// Record panics and route local
	defer func() {
		if fatalError := recover(); fatalError != nil {
			stack := debug.Stack()
			err = fmt.Errorf("panic in shard inter-process fragment instance sender: %v\n%s", fatalError, stack)
		}
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		err = fmt.Errorf("failed to connect to remote shard: %w", err)
		return
	}
	defer func() {
		lerr := conn.Close()
		if lerr != nil && err == nil {
			err = fmt.Errorf("failed closing connection: %w", lerr)
		}
	}()

	// New fipr session
	session, err := fipr.New(conn, wrappers.GetSharedSecret())
	if err != nil {
		err = fmt.Errorf("session creation failed: %w", err)
		return
	}

	err = session.SendStart(messageID)
	if err != nil {
		return
	}

	// Session setup
	err = session.SendOnBehalfOf(remoteAddress.String())
	if err != nil {
		return
	}
	draining, err := session.SendShardCheck()
	if err != nil {
		return
	}
	msgExists, err := session.SendMessageCheck()
	if err != nil {
		return
	}

	// Shard is only accepting existing messages
	if !msgExists && draining {
		rerouteLocally = true
		return
	}
	// Shard is accepting new or existing messages

	// Using existing serialization from main protocol (using signature id from original packet)
	rawPayload, err := protocol.ConstructPayload(fragment, fragment.SignatureID)
	if err != nil {
		err = fmt.Errorf("failed to validate fragment: %w", err)
		return
	}
	payload, err := protocol.ConstructInnerPayload(rawPayload)
	if err != nil {
		err = fmt.Errorf("failed to serialize fragment: %w", err)
		return
	}

	err = session.SendFragment(payload)
	if err != nil {
		return
	}

	// Success
	return
}
