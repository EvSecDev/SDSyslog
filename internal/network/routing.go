package network

import (
	"fmt"
	"net"
	"strings"
)

// Determines the interface used to reach a given destination address
func getInterfaceForDestination(destination string) (iface *net.Interface, err error) {
	rawIP := strings.TrimPrefix(destination, "[")
	rawIP = strings.TrimSuffix(rawIP, "]")

	// Parse the destination address
	destAddr := net.ParseIP(rawIP)
	if destAddr == nil {
		err = fmt.Errorf("invalid destination address: %s", destination)
		return
	}

	var formattedIP string
	if destAddr.To16() != nil {
		formattedIP = "[" + destAddr.To16().String() + "]"
	} else {
		formattedIP = rawIP
	}

	// Quick dial to see what source interface the system would use
	conn, dialErr := net.Dial("udp", formattedIP+":0")
	if dialErr != nil {
		err = fmt.Errorf("failed to find interface for destination %s: %w", formattedIP, dialErr)
		return
	}
	defer conn.Close()

	// Get the interface for the local half of the connection
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	iface, err = getInterfaceForAddress(localAddr.IP.String())
	return
}
