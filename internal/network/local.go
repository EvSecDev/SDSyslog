// Package containing logic for interacting with the local operating system network stack
package network

import (
	"fmt"
	"net"
	"strings"
	"time"
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

	// Short circuit loopback (no auto select)
	if allowLoopback {
		sourceSocket, err = ParseUDPAddress("localhost", 0)
		return
	}

	raddr := &net.UDPAddr{
		IP:   destAddress,
		Port: 9, // discard port, arbitrary
	}

	var selectedSourceAddr net.IP
	var gotValidSource bool

	// Early startup can race network interface/route setup, retry until ready with 2 limiters
	waitDuration := 500 * time.Microsecond
	maxWaitDuration := 10 * time.Second
	maxRetries := 30
	startTime := time.Now()
	maxTotalRetryDuration := 1 * time.Minute
	deadline := startTime.Add(maxTotalRetryDuration)

	var retryCount int
	for retryCount = range maxRetries {
		// Stop after deadline
		if time.Now().After(deadline) {
			break
		}

		// Dial UDP (no actual network traffic generated) to see what route table selects as source
		var conn *net.UDPConn
		conn, err = net.DialUDP(network, nil, raddr)
		if err != nil {
			err = fmt.Errorf("dial failed: %w", err)
			return
		}
		localAddr := conn.LocalAddr()
		_ = conn.Close()

		udpAddr, ok := localAddr.(*net.UDPAddr)
		if !ok {
			err = fmt.Errorf("unexpected local addr type: %T", localAddr)
			return
		}

		selectedSourceAddr = udpAddr.IP
		if selectedSourceAddr == nil {
			err = fmt.Errorf("no local IP selected")
			return
		}

		// Got valid source
		if !selectedSourceAddr.IsLoopback() {
			gotValidSource = true
			break
		}

		// Selected loopback, wait and retry for real address
		time.Sleep(waitDuration)
		waitDuration = waitDuration * 2
		if waitDuration >= maxWaitDuration {
			waitDuration = maxWaitDuration
		}
	}
	if !gotValidSource {
		err = fmt.Errorf("kernel selected loopback source (%s) for non-loopback destination (%s) after retrying %d time(s) over %s",
			selectedSourceAddr, destAddress, retryCount+1, maxTotalRetryDuration.String())
		return
	}

	sourceSocket, err = ParseUDPAddress(selectedSourceAddr.String(), 0)
	return
}
