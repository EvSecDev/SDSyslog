package fsops

import (
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

type CapMode uint8

type CapSet struct {
	PermittedLow    uint32
	PermittedHigh   uint32
	InheritableLow  uint32
	InheritableHigh uint32
}

// Gets Linux capabilities currently set on a file
func GetCapabilities(path string) (mode CapMode, caps []uint, err error) {
	buf := make([]byte, 20)
	_, err = unix.Getxattr(path, xattrName, buf)
	if err != nil && !errors.Is(err, unix.ENODATA) {
		err = fmt.Errorf("getxattr: %w", err)
		return
	} else if err != nil && errors.Is(err, unix.ENODATA) {
		// Exclude no set capabilities as error (also nothing to decode)
		err = nil
		return
	}

	mode, caps, err = decodeVfsCapDataV2(buf)
	return
}

// Sets Linux capabilities on file
func SetCapabilities(path string, mode CapMode, caps ...uint) (err error) {
	if len(caps) == 0 {
		err = fmt.Errorf("no capabilities provided")
		return
	}

	buf := encodeVfsCapDataV2(mode, caps...)

	// Example from `strace setcap cap_sys_resource,cap_bpf=eip /path/to/binary`
	//                             Literal for Caps       Value                                          Size   Flags
	// setxattr("/path/to/binary", "security.capability", "\1\0\0\2\0\0\0\1\0\0\0\1\200\0\0\0\200\0\0",  20,    0     ) = 0
	err = unix.Setxattr(path, xattrName, buf, 0)
	if err != nil {
		err = fmt.Errorf("setxattr: %w", err)
		return
	}

	return
}

func encodeVfsCapDataV2(mode CapMode, caps ...uint) (payload []byte) {
	var cs CapSet

	for _, cap := range caps {
		if cap < 32 {
			if mode&CapPermitted != 0 {
				cs.PermittedLow |= 1 << cap
			}
			if mode&CapInheritable != 0 {
				cs.InheritableLow |= 1 << cap
			}
		} else {
			shift := cap - 32
			if mode&CapPermitted != 0 {
				cs.PermittedHigh |= 1 << shift
			}
			if mode&CapInheritable != 0 {
				cs.InheritableHigh |= 1 << shift
			}
		}
	}

	magic := uint32(vfsCapRevision2)
	if mode&CapEffective != 0 {
		magic |= vfsCapFlagsEffective
	}

	payload = make([]byte, 20)

	binary.LittleEndian.PutUint32(payload[0:4], magic)
	binary.LittleEndian.PutUint32(payload[4:8], cs.PermittedLow)
	binary.LittleEndian.PutUint32(payload[8:12], cs.InheritableLow)
	binary.LittleEndian.PutUint32(payload[12:16], cs.PermittedHigh)
	binary.LittleEndian.PutUint32(payload[16:20], cs.InheritableHigh)
	return
}

func decodeVfsCapDataV2(payload []byte) (mode CapMode, caps []uint, err error) {
	if len(payload) < 20 {
		err = fmt.Errorf("invalid vfs_cap_data_v2 length: %d", len(payload))
		return
	}

	magic := binary.LittleEndian.Uint32(payload[0:4])

	version := magic & 0xFFFF_FFF0
	if version != vfsCapRevision2 {
		err = fmt.Errorf("unsupported capability version: %x", version)
		return
	}

	if magic&vfsCapFlagsEffective != 0 {
		mode |= CapEffective
	}

	permittedLow := binary.LittleEndian.Uint32(payload[4:8])
	inheritableLow := binary.LittleEndian.Uint32(payload[8:12])
	permittedHigh := binary.LittleEndian.Uint32(payload[12:16])
	inheritableHigh := binary.LittleEndian.Uint32(payload[16:20])

	addCaps := func(mask uint32, base uint) {
		for i := 0; i < 32; i++ {
			if mask&(1<<i) != 0 {
				caps = append(caps, base+uint(i))
			}
		}
	}

	if mode&CapPermitted != 0 {
		addCaps(permittedLow, 0)
		addCaps(permittedHigh, 32)
	}

	_ = inheritableLow
	_ = inheritableHigh

	return
}
