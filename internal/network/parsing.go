package network

import (
	"fmt"
	"net"
	"sdsyslog/internal/crypto/random"
	"strconv"
)

// Parses and resolves address and port into UDP socket.
// If port supplied is 0, a random port in ephemeral range is used.
// If address string also has a port, the explicitly provided port is ignored if 0
func ParseUDPAddress(address string, port int) (socket *net.UDPAddr, err error) {
	if address == "" {
		err = fmt.Errorf("no address supplied")
		return
	}

	var host string
	var embeddedPort int
	var hasEmbeddedPort bool

	// Try splitting host:port
	splitHost, splitPort, err := net.SplitHostPort(address)
	if err == nil {
		host = splitHost
		if splitPort != "" {
			var portNum int
			portNum, err = strconv.Atoi(splitPort)
			if err != nil {
				err = fmt.Errorf("port is not a number in address %q: %w", address, err)
				return
			}
			if portNum < 0 || portNum > 65535 {
				err = fmt.Errorf("port out of range in address %q", address)
				return
			}

			embeddedPort = portNum
			hasEmbeddedPort = true
		}
	} else {
		// No port in address, treat entire string as host
		host = address
	}

	// Decide final port
	var finalPort int
	switch {
	case port > 0 && port <= 65535:
		// Explicit port always wins
		finalPort = port
	case hasEmbeddedPort:
		// Address string has port
		finalPort = embeddedPort
	case port == 0:
		// Neither provided, generate ephemeral
		finalPort, err = random.NumberInRange(EphemeralPortMin, EphemeralPortMax)
		if err != nil {
			err = fmt.Errorf("failed to generate random number: %w", err)
			return
		}
	default:
		err = fmt.Errorf("invalid port %d", port)
		return
	}

	fullAddr := net.JoinHostPort(host, strconv.Itoa(finalPort))

	socket, err = net.ResolveUDPAddr("udp", fullAddr)
	if err != nil {
		err = fmt.Errorf("failed address resolution for %q: %w", fullAddr, err)
		return
	}

	return
}
