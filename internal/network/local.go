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
	destIPVersion := "udp4"
	if destAddress.To4() == nil {
		destIPVersion = "udp6"
	}

	// If destination is loopback, allow loopback
	allowLoopback := destAddress.IsLoopback()

	// Short circuit loopback (no auto select)
	if allowLoopback {
		// Manually decide IP version since localhost could resolve incorrectly
		var sourceAddr string
		switch destIPVersion {
		case "udp4":
			sourceAddr = "127.0.0.1"
		case "udp6":
			sourceAddr = "::1"
		}
		sourceSocket, err = ParseUDPAddress(sourceAddr, 0)
		return
	}

	raddr := &net.UDPAddr{
		IP:   destAddress,
		Port: 9, // discard port, arbitrary (but 9 is a good sentinel)
	}

	// Early startup can race network interface/route setup, retry until ready with 2 limiters
	waitDuration := 500 * time.Microsecond
	maxWaitDuration := 10 * time.Second
	maxRetries := 30
	startTime := time.Now()
	maxTotalRetryDuration := 1 * time.Minute
	deadline := startTime.Add(maxTotalRetryDuration)

	var selectedSourceAddr net.IP
	var gotValidSource bool

	var retryCount int
	for retryCount = range maxRetries {
		// Stop after deadline
		if time.Now().After(deadline) {
			break
		}
		// Increasing delay after initial failure
		if retryCount > 1 {
			time.Sleep(waitDuration)
			waitDuration = waitDuration * 2

			if waitDuration >= maxWaitDuration {
				waitDuration = maxWaitDuration
			}
		}

		// Dial UDP (no actual network traffic generated) to see what route table selects as source
		var conn *net.UDPConn
		conn, err = net.DialUDP(destIPVersion, nil, raddr)
		if err != nil {
			err = fmt.Errorf("dial failed: %w", err)
			continue
		}
		localAddr := conn.LocalAddr()
		_ = conn.Close()

		udpAddr, ok := localAddr.(*net.UDPAddr)
		if !ok {
			err = fmt.Errorf("unexpected local addr type: %T", localAddr)
			return
		}
		if udpAddr == nil || udpAddr.IP == nil {
			err = fmt.Errorf("dial connection did not contain source address IP")
			return
		}

		if udpAddr.IP.IsLoopback() {
			// Selected loopback, retry for real address
			err = fmt.Errorf("kernel selected loopback source (%s) for non-loopback destination (%s)",
				selectedSourceAddr, destAddress)
			continue
		}

		// Got valid source
		selectedSourceAddr = udpAddr.IP
		gotValidSource = true
		break
	}
	if !gotValidSource {
		// Wrap whichever error was set last during the retry loop
		err = fmt.Errorf("failed to select source address after retrying %d time(s) over %s: %w",
			retryCount+1, maxTotalRetryDuration.String(), err)
		return
	}

	sourceSocket, err = ParseUDPAddress(selectedSourceAddr.String(), 0)
	return
}
