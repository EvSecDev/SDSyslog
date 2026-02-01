package network

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sdsyslog/internal/ebpf"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Creates new udp connection object for a port that is already listening.
// Attempts to set socket with hooks to ebpf program if available.
func ReuseUDPPort(port int) (conn *net.UDPConn, err error) {
	// Using x/sys/unix package for more up-to-date syscall numbers
	cfg := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var ctrlErr error
			c.Control(func(fd uintptr) {
				// Always set SO_REUSEADDR and SO_REUSEPORT
				err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				if err != nil {
					ctrlErr = fmt.Errorf("SO_REUSEADDR failed: %w", err)
					return
				}
				err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				if err != nil {
					ctrlErr = fmt.Errorf("SO_REUSEPORT failed: %w", err)
					return
				}

				// Attempt to attach eBPF program if pinned
				prog, err := os.Open(ebpf.KernelSocketRouteFunc)
				if err != nil {
					// eBPF not available; silently fall back to normal reuseport
					return
				}
				defer prog.Close()

				progFD := int(prog.Fd())
				err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_ATTACH_REUSEPORT_EBPF, progFD)
				if err != nil {
					// Kernel does not support or failed; fallback to normal reuseport
					return
				}
			})
			return ctrlErr
		},
	}

	addr := net.UDPAddr{Port: port}
	pc, err := cfg.ListenPacket(context.Background(), "udp", addr.String())
	if err != nil {
		err = fmt.Errorf("failed to listen on new reused connection: %w", err)
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
		err = fmt.Errorf("failed to listen on reused tcp port: %w", err)
		return
	}

	return
}

// Waits until socket receive buffer is 0 three times in a row, with retries and timeout.
// Only tracks the next available datagram to read, not entire buffer.
// Returned remaining bytes is a `AT LEAST` value.
// Should only be used to verify that the socket is empty.
// NOT reliable unless used in conjunction with eBPF draining program.
func WaitUntilEmptySocket(conn *net.UDPConn) (remainingBytes int, err error) {
	const successfulStreakCount int = 3

	file, err := conn.File()
	if err != nil {
		return
	}
	defer file.Close()

	fd := int(file.Fd())

	// Initial backoff duration
	backoffDuration := 50 * time.Millisecond

	// Max backoff duration
	maxBackoff := 1 * time.Second

	// Maximum number of iterations
	maxIterations := 6

	// Track consecutive values at 0
	zeroStreak := 0

	var fionread uint
	switch runtime.GOOS {
	case "linux":
		fionread = 0x541B
	case "darwin", "freebsd", "openbsd", "netbsd":
		fionread = 0x4004667F
	default:
		return
	}

	// Retry loop with exponential backoff
	for i := 0; i < maxIterations; i++ {
		remainingBytes, err = unix.IoctlGetInt(fd, fionread)
		if err != nil {
			return
		}

		if remainingBytes == 0 {
			zeroStreak++
			if zeroStreak >= successfulStreakCount {
				return
			}
		} else {
			// Reset streak if value is non-zero
			zeroStreak = 0
		}

		// Sleep for the backoff duration
		time.Sleep(backoffDuration)

		// Increase the backoff duration exponentially
		if backoffDuration < maxBackoff {
			backoffDuration *= 2
			if backoffDuration > maxBackoff {
				backoffDuration = maxBackoff
			}
		}
	}

	// Timed out
	return
}
