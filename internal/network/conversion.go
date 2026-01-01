package network

import (
	"encoding/binary"
	"fmt"
	"net"
)

// IPToTwoInts converts IPv4/IPv6 strings into two positive ints.
// IPv4: hi = IPv4 number, lo = 0;
// IPv6: hi = first half of IPv6, lo = second half of IPv6;
func IPtoIntegers(ipStr string) (hi int, lo int, err error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		err = fmt.Errorf("invalid IP: %s", ipStr)
		return
	}

	// Convert to bytes as possible IPv4 or IPv6
	v4Address := ip.To4()
	v6Address := ip.To16()

	// Set hi/lo integers based on type
	if v4Address != nil {
		// IPv4
		hi = int(binary.BigEndian.Uint32(v4Address))
		lo = 0
	} else if v6Address != nil {
		// IPv6
		hi = int(binary.BigEndian.Uint64(v6Address[0:8]))
		lo = int(binary.BigEndian.Uint64(v6Address[8:16]))
	} else {
		err = fmt.Errorf("invalid IP format: %s", ipStr)
		return
	}

	// Ensure hi/lo integers are positive
	if hi < 0 {
		hi = -hi
	}
	if lo < 0 {
		lo = -lo
	}

	return
}
