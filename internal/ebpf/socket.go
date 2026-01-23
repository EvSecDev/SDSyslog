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
	if runtime.GOOS != "linux" {
		return
	}

	file, err := conn.File()
	if err != nil {
		return
	}
	defer file.Close()

	fd := int(file.Fd())

	if fd <= 2 {
		err = fmt.Errorf("unsupported fd for socket")
		return
	}

	cookie, err = unix.GetsockoptUint64(fd, unix.SOL_SOCKET, unix.SO_COOKIE)
	if err != nil {
		err = fmt.Errorf("getsockopt failed: %v", err)
		return
	}

	return
}

// Mark a given socket identifier (cookie) as draining.
// Uses eBPF program to prevent kernel from sending additional data to sockets buffer.
func MarkSocketDraining(pinnedMapPath string, socketCookie uint64) (err error) {
	if runtime.GOOS != "linux" {
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
		err = fmt.Errorf("failed to load eBPF map: %v", err)
		return
	}
	defer socketMap.Close()

	err = socketMap.Put(socketCookie, uint8(global.DrainSocket))
	if err != nil {
		err = fmt.Errorf("failed to mark socket draining: %v\n", err)
		return
	}

	return
}
