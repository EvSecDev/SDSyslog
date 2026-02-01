package network

import (
	"fmt"
	"net"
	"strings"
)

// Retrieves total overhead for given IP and protocol
func getTransportOverhead(destination string, protocol string) (overhead int, err error) {
	const ip4Overhead int = 60
	const ip6Overhead int = 80
	const udpOverhead int = 8

	var transportLayerOverhead int
	switch protocol {
	case "udp":
		transportLayerOverhead = udpOverhead
	}

	if strings.Contains(destination, ":") {
		overhead = ip6Overhead + transportLayerOverhead
	} else if strings.Contains(destination, ".") {
		overhead = ip4Overhead + transportLayerOverhead
	} else {
		err = fmt.Errorf("unsupported destination address '%v'", destination)
		return
	}

	return
}

// Determines the maximum UDP payload size for the sender mode based on the destination address
func FindSendingMaxUDPPayload(destination string) (maxPayloadSize int, err error) {
	var destinationIP string
	host, _, err := net.SplitHostPort(destination)
	if err != nil {
		// If it's not in host:port format, assume it's just a host or IP
		destinationIP = destination
		err = nil
	} else {
		destinationIP = host
	}

	// Default to ethernet standard MTU if no other MTUis found
	const defaultMTU int = 1500

	// Precalculate overhead
	overhead, err := getTransportOverhead(destinationIP, "udp")
	if err != nil {
		err = fmt.Errorf("failed to retrieve transport layer overhead: %w", err)
		return
	}

	// If destination is loopback, will get the MTU of a loopback interface
	var destIsLoopback bool
	if strings.HasPrefix(destinationIP, "127.") || destinationIP == "[::1]" {
		destIsLoopback = true
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	// Retrieve interface MTUs
	var commonMTU int
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 && destIsLoopback {
			// Don't attempt to find common when requested destination is loopback
			break
		} else if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Identify all identical MTUs
		if commonMTU == 0 {
			commonMTU = iface.MTU
		} else if commonMTU != iface.MTU {
			// MTUs are not the same across non-loopback interfaces
			commonMTU = 0
			break
		}
	}

	// Determine MTU by route table if we couldn't find a common one
	var mtu int
	if commonMTU == 0 {
		var iface *net.Interface
		iface, err = getInterfaceForDestination(destinationIP)
		if err != nil {
			// No route found - fail early
			return
		}
		mtu = iface.MTU
	} else {
		// All non-loopback interfaces have the same MTU
		mtu = commonMTU
	}

	// Safety check - assign default
	if mtu <= 0 {
		mtu = defaultMTU
	}

	// Return found MTU accounting for overhead of requested destination
	maxPayloadSize = mtu - overhead
	return
}
