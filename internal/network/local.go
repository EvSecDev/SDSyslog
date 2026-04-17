// Package containing logic for interacting with the local operating system network stack
package network

import (
	"fmt"
	"net"
	"strings"
)

// Retrieves the network interface corresponding to a specific address
func getInterfaceForAddress(address string) (iface *net.Interface, err error) {
	address = strings.TrimPrefix(address, "[")
	address = strings.TrimSuffix(address, "]")

	// Get all network interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	// Loop through all interfaces and check for the one matching the address
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if ok && ipNet.IP.String() == address {
				return &iface, nil
			}
		}
	}

	err = fmt.Errorf("no matching interface found for address %v", address)
	return
}

// Finds local source address given destination
func GetLocalIPForDestination(destAddress net.IP) (sourceSocket *net.UDPAddr, err error) {
	if destAddress == nil {
		err = fmt.Errorf("destination IP is nil")
		return
	}

	// Determine network type
	network := "udp4"
	if destAddress.To4() == nil {
		network = "udp6"
	}

	// If destination is loopback, allow loopback
	allowLoopback := destAddress.IsLoopback()

	raddr := &net.UDPAddr{
		IP:   destAddress,
		Port: 9, // discard port, arbitrary
	}

	conn, err := net.DialUDP(network, nil, raddr)
	if err != nil {
		err = fmt.Errorf("dial failed: %w", err)
		return
	}
	defer func() {
		_ = conn.Close()
	}()

	localAddr := conn.LocalAddr()
	udpAddr, ok := localAddr.(*net.UDPAddr)
	if !ok {
		err = fmt.Errorf("unexpected local addr type: %T", localAddr)
		return
	}

	sourceAddress := udpAddr.IP
	if sourceAddress == nil {
		err = fmt.Errorf("no local IP selected")
		return
	}

	// No loopback unless dst is loopback
	if sourceAddress.IsLoopback() && !allowLoopback {
		err = fmt.Errorf("kernel selected loopback source (%s) for non-loopback destination (%s)", sourceAddress, destAddress)
		return
	}

	sourceSocket, err = ParseUDPAddress(sourceAddress.String(), 0)
	return
}
