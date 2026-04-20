package fsops

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestReloadSigningKeys(t *testing.T) {
	tests := []struct {
		name     string
		mode     CapMode
		caps     []uint
		expected []byte
	}{
		{
			name: "CAP_SYS_RESOURCE only (eip)",
			mode: CapPermitted | CapInheritable | CapEffective,
			caps: []uint{CapSYSResource},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "NET_BIND_SERVICE (ep)",
			mode: CapPermitted | CapEffective,
			caps: []uint{CapNetBindService},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x00, 0x04, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "CAP_SYS_RESOURCE + CAP_BPF (eip)",
			mode: CapPermitted | CapInheritable | CapEffective,
			caps: []uint{CapSYSResource, CapBPF},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x01,
				0x00, 0x00, 0x00, 0x01,
				0x80, 0x00, 0x00, 0x00,
				0x80, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "CAP_BPF only (ep)",
			mode: CapPermitted | CapEffective,
			caps: []uint{CapBPF},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x80, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "SYSLOG (eip)",
			mode: CapPermitted | CapInheritable | CapEffective,
			caps: []uint{CapSyslog},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x04, 0x00, 0x00, 0x00, // 34 - 32 = 2 → 1<<2 = 0x4, but shifted into high word layout
				0x04, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "NET_ADMIN + NET_RAW (ep)",
			mode: CapPermitted | CapEffective,
			caps: []uint{CapNetAdmin, CapNetRaw},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x00, 0x30, 0x00, 0x00, // bits 12 + 13
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
			},
		},
		{
			name: "DAC_OVERRIDE + BPF (eip)",
			mode: CapPermitted | CapInheritable | CapEffective,
			caps: []uint{CapDACOverride, CapBPF},
			expected: []byte{
				0x01, 0x00, 0x00, 0x02,
				0x02, 0x00, 0x00, 0x00, // DAC override = bit 1
				0x02, 0x00, 0x00, 0x00,
				0x80, 0x00, 0x00, 0x00,
				0x80, 0x00, 0x00, 0x00,
			},
		},
	}

	formatBytes := func(b []byte) (formatted string) {
		out := make([]string, len(b))
		for i, v := range b {
			out[i] = fmt.Sprintf("%02x", v)
		}
		formatted = strings.Join(out, " ")
		return
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeVfsCapDataV2(tt.mode, tt.caps...)

			if !bytes.Equal(got, tt.expected) {
				t.Fatalf(
					"cap syscall payload mismatch\nGOT:  %v\nWANT: %v",
					formatBytes(got),
					formatBytes(tt.expected),
				)
			}
		})
	}
}
