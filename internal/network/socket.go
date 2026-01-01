package network

import (
	"context"
	"fmt"
	"net"
	"syscall"
)

// Creates new udp connection object for a port that is already listening
func ReuseUDPPort(port int) (conn *net.UDPConn, err error) {
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if err != nil {
					return
				}
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, 0x0F /* SO_REUSEPORT */, 1)
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
