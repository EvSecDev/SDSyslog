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
