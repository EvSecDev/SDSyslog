package ebpf

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sdsyslog/internal/fsops"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

//go:embed static-files/*
var byteCodeFS embed.FS

// Takes compiled eBPF bytecode and loads it into the kernel.
// It pins the draining_sockets map and reference to the current program in sys bpffs path.
func LoadProgram(ctx context.Context) (err error) {
	if runtime.GOOS != global.GOOSLinux {
		logctx.LogStdWarn(ctx, "eBPF is not supported on platform %s\n", runtime.GOOS)
		return
	}

	// Ensure kernel supports BTF
	_, err = os.Stat("/sys/kernel/btf/vmlinux")
	if err != nil && os.IsNotExist(err) {
		// No-op when not supported
		logctx.LogStdWarn(ctx, "eBPF is not supported on current kernel\n")
		err = nil
		return
	} else if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed to check existence of BTF: %w", err)
		return
	}

	// Ensure binary file has proper capabilities set for non-root users
	if os.Geteuid() != 0 {
		var selfExe string
		selfExe, err = os.Executable()
		if err != nil {
			err = fmt.Errorf("failed to retrieve self executable file path: %w", err)
			return
		}
		var mode fsops.CapMode
		var caps []uint
		mode, caps, err = fsops.GetCapabilities(selfExe)
		if err != nil {
			err = fmt.Errorf("failed to retrieve capabilities for self executable: %w", err)
			return
		}

		var bpfCapSet, sysResourceCapSet bool
		for _, cap := range caps {
			if cap == fsops.CapBPF {
				bpfCapSet = true
			}
			if cap == fsops.CapSYSResource {
				sysResourceCapSet = true
			}
		}
		modeIsCorrect := mode == fsops.CapEffective|fsops.CapInheritable|fsops.CapPermitted
		if !bpfCapSet || !sysResourceCapSet || !modeIsCorrect {
			// No-op (log for informative)
			logctx.LogStdInfo(ctx, "Executable file is missing capability required to use eBPF socket draining program: CAP_SYS_RESOURCE,CAP_BPF=eip\n")
			logctx.LogStdInfo(ctx, "eBPF socket draining will not be enabled (may cause dropped packets on scaling/upgrade events)\n")
			return
		}
	}

	ebpfByteCode, err := byteCodeFS.ReadFile("static-files/socket.o")
	if err != nil {
		err = fmt.Errorf("read bytecode: %w", err)
		return
	}

	err = unix.Setrlimit(unix.RLIMIT_MEMLOCK, &unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	})
	if err != nil {
		err = fmt.Errorf("set resource limit: %w", err)
		return
	}

	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(ebpfByteCode))
	if err != nil {
		err = fmt.Errorf("load eBPF spec: %w", err)
		return
	}

	// Loading into kernel
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		err = fmt.Errorf("load eBPF collection: %w", err)
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
			err = fmt.Errorf("bpffs was not mounted and mount attempt failed: %w", err)
			return
		}
	} else if err != nil {
		err = nil
		return
	}

	_, err = os.Stat(KernelDrainMapPath)
	if err != nil && os.IsNotExist(err) {
		// Only pin once per host boot
		drainingMap, ok := coll.Maps[DrainMapName]
		if !ok {
			err = fmt.Errorf("map %s not found in collection", DrainMapName)
			return
		}

		err = drainingMap.Pin(KernelDrainMapPath)
		if err != nil && !errors.Is(err, os.ErrExist) {
			err = fmt.Errorf("pin map: %w", err)
			return
		}
	}

	// Unpin old versions
	_, err = os.Stat(KernelSocketRouteFunc)
	if err == nil {
		err = os.Remove(KernelSocketRouteFunc)
		if err != nil && !os.IsNotExist(err) {
			err = fmt.Errorf("failed to remove old bpffs function file: %w", err)
			return
		}
	}

	prog, ok := coll.Programs[DrainFuncName]
	if !ok {
		err = fmt.Errorf("program %s not found in collection", DrainFuncName)
		return
	}

	// Pin newest
	err = prog.Pin(KernelSocketRouteFunc)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("pin function: %w", err)
		return
	}

	return
}
