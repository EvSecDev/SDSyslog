package network

import (
	"context"
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// Creates new udp connection object for a port that is already listening
func ReuseUDPPort(port int) (conn *net.UDPConn, err error) {
	// Using x/sys/unix package for more up-to-date syscall numbers
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				if err != nil {
					return
				}
				err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
			return err
		},
	}

	addr := net.UDPAddr{Port: port}
	pc, err := cfg.ListenPacket(context.Background(), "udp", addr.String())
	if err != nil {
		err = fmt.Errorf("failed to listen on new reused connection: %v", err)
		return
	}
	conn = pc.(*net.UDPConn)
	return
}

// Creates new TCP listener object for a port that is already listening
func ReuseTCPPort(addr string) (conn net.Listener, err error) {
	// Using x/sys/unix package for more up-to-date syscall numbers
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				// Allow port reuse
				err = unix.SetsockoptInt(
					int(fd),
					unix.SOL_SOCKET,
					unix.SO_REUSEADDR,
					1,
				)
				if err != nil {
					return
				}

				// Allow multiple active listeners
				err = unix.SetsockoptInt(
					int(fd),
					unix.SOL_SOCKET,
					unix.SO_REUSEPORT,
					1,
				)
			})
			return err
		},
	}

	conn, err = cfg.Listen(context.Background(), "tcp", addr)
	if err != nil {
		err = fmt.Errorf("failed to listen on reused tcp port: %v", err)
		return
	}

	return
}
