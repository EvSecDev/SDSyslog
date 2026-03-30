package ebpf

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sdsyslog/internal/global"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

// Retrieve unique identifier (cookie) for a given socket file descriptor.
func GetSocketCookie(conn *net.UDPConn) (cookie uint64, err error) {
	if runtime.GOOS != global.GOOSLinux {
		return
	}

	file, err := conn.File()
	if err != nil {
		return
	}
	defer func() {
		lerr := file.Close()
		if lerr != nil && err == nil {
			err = fmt.Errorf("failed to close connection: %w", lerr)
		}
	}()

	fd := int(file.Fd())

	if fd <= 2 {
		err = fmt.Errorf("unsupported fd for socket")
		return
	}

	cookie, err = unix.GetsockoptUint64(fd, unix.SOL_SOCKET, unix.SO_COOKIE)
	if err != nil {
		err = fmt.Errorf("getsockopt failed: %w", err)
		return
	}

	return
}

// Mark a given socket identifier (cookie) as draining.
// Uses eBPF program to prevent kernel from sending additional data to sockets buffer.
func MarkSocketDraining(pinnedMapPath string, socketCookie uint64) (err error) {
	if runtime.GOOS != global.GOOSLinux {
		return
	}

	if pinnedMapPath == "" {
		err = fmt.Errorf("map path empty")
		return
	}

	_, err = os.Stat(pinnedMapPath)
	if err != nil && (os.IsNotExist(err) || os.IsPermission(err)) {
		// No-op when map is not available
		err = nil
		return
	}

	socketMap, err := ebpf.LoadPinnedMap(pinnedMapPath, nil)
	if err != nil {
		err = fmt.Errorf("failed to load eBPF map: %w", err)
		return
	}
	defer func() {
		lerr := socketMap.Close()
		if lerr != nil && err == nil {
			err = fmt.Errorf("failed to close socket map: %w", lerr)
		}
	}()

	err = socketMap.Put(socketCookie, uint8(DrainSocket))
	if err != nil {
		err = fmt.Errorf("failed to mark socket draining: %w", err)
		return
	}

	return
}
