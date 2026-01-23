package ebpf

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/global"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

//go:embed static-files/*
var byteCodeFS embed.FS

// Takes compiled eBPF bytecode and loads it into the kernel.
// It pins the draining_sockets map and reference to the current program in sys bpffs path.
func LoadProgram() (err error) {
	if runtime.GOOS != "linux" {
		return
	}

	// Ensure kernel supports BTF
	_, err = os.Stat("/sys/kernel/btf/vmlinux")
	if os.IsNotExist(err) {
		// No-op when not supported
		err = nil
		return
	}

	// Must run as root
	if os.Geteuid() != 0 {
		// No-op
		return
	}

	ebpfByteCode, err := byteCodeFS.ReadFile("static-files/socket.o")
	if err != nil {
		err = fmt.Errorf("read bytecode: %v", err)
		return
	}

	err = unix.Setrlimit(unix.RLIMIT_MEMLOCK, &unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	})
	if err != nil {
		err = fmt.Errorf("set resource limit: %v", err)
		return
	}

	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(ebpfByteCode))
	if err != nil {
		err = fmt.Errorf("load eBPF spec: %v", err)
		return
	}

	// Loading into kernel
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		err = fmt.Errorf("load eBPF collection: %v", err)
		return
	}

	// Ensure the bpf file system is present for pinning
	_, err = os.Stat("/sys/fs/bpf")
	if err != nil && os.IsNotExist(err) {
		err = unix.Mount("bpffs",
			"/sys/fs/bpf", "bpf",
			0, "",
		)
		if err != nil {
			err = fmt.Errorf("bpffs was not mounted and mount attempt failed: %v", err)
			return
		}
	} else if err != nil {
		err = nil
		return
	}

	_, err = os.Stat(global.KernelDrainMapPath)
	if err != nil && os.IsNotExist(err) {
		// Only pin once per host boot
		drainingMap, ok := coll.Maps[global.DrainMapName]
		if !ok {
			err = fmt.Errorf("map %s not found in collection", global.DrainMapName)
			return
		}

		err = drainingMap.Pin(global.KernelDrainMapPath)
		if err != nil && !errors.Is(err, os.ErrExist) {
			err = fmt.Errorf("pin map: %v", err)
			return
		}
	}

	// Unpin old versions
	_, err = os.Stat(global.KernelSocketRouteFunc)
	if err == nil {
		err = os.Remove(global.KernelSocketRouteFunc)
		if err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("failed to remove old bpffs function file: %v", err)
			return
		}
	}

	prog, ok := coll.Programs[global.DrainFuncName]
	if !ok {
		err = fmt.Errorf("program %s not found in collection", global.DrainFuncName)
		return
	}

	// Pin newest
	err = prog.Pin(global.KernelSocketRouteFunc)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("pin function: %v", err)
		return
	}

	return
}
