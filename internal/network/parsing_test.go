package network

import (
	"net"
	"sdsyslog/internal/tests/utils"
	"testing"
)

func TestParseUDPAddress(t *testing.T) {
	tests := []struct {
		name           string
		address        string
		port           int
		expectedSocket *net.UDPAddr
		expectedError  string
	}{
		{
			name:          "empty address",
			address:       "",
			port:          0,
			expectedError: "no address supplied",
		},
		{
			name:    "host only, no port -> random ephemeral",
			address: "127.0.0.1",
			port:    0,
			expectedSocket: &net.UDPAddr{
				IP: net.ParseIP("127.0.0.1"),
				// Port checked separately (range)
			},
		},
		{
			name:    "max valid port",
			address: "127.0.0.1",
			port:    65535,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 65535,
			},
		},
		{
			name:    "host with explicit port",
			address: "127.0.0.1",
			port:    8080,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 8080,
			},
		},
		{
			name:    "embedded port only",
			address: "127.0.0.1:9000",
			port:    0,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 9000,
			},
		},
		{
			name:    "embedded port overridden by explicit port",
			address: "127.0.0.1:9000",
			port:    8080,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 8080,
			},
		},
		{
			name:          "embedded port out of range",
			address:       "127.0.0.1:70000",
			expectedError: "port out of range",
		},
		{
			name:          "invalid embedded port",
			address:       "127.0.0.1:abc",
			port:          0,
			expectedError: "port is not a number in address",
		},
		{
			name:          "port out of range explicit",
			address:       "127.0.0.1",
			port:          70000,
			expectedError: "invalid port 70000",
		},
		{
			name:          "invalid host",
			address:       "!!!invalid!!!",
			port:          1234,
			expectedError: "failed address resolution for",
		},
		{
			name:    "ipv6 with embedded port",
			address: "[::1]:5353",
			port:    0,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("::1"),
				Port: 5353,
			},
		},
		{
			name:    "ipv6 override port",
			address: "[::1]:5353",
			port:    9999,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("::1"),
				Port: 9999,
			},
		},
		{
			name:    "hostname resolution",
			address: "localhost:8080",
			port:    0,
			expectedSocket: &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"), // may vary on some systems
				Port: 8080,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socket, err := ParseUDPAddress(tt.address, tt.port)
			matches, err := utils.MatchErrorString(err, tt.expectedError)
			if err != nil {
				t.Fatalf("%v", err)
			} else if matches {
				return
			}

			if !equalUDPAddr(tt.expectedSocket, socket) {
				if socket == nil {
					t.Errorf("expected socket %+v but got nil socket", *tt.expectedSocket)
				} else if tt.expectedSocket == nil {
					t.Errorf("expected nil socket but got socket %+v", *socket)
				} else {
					t.Errorf("expected socket %+v but got socket %+v", *tt.expectedSocket, *socket)
				}
			}
		})
	}
}

func equalUDPAddr(expectedSocket, gotSocket *net.UDPAddr) (isEqual bool) {
	if expectedSocket == nil || gotSocket == nil {
		isEqual = expectedSocket == gotSocket
		return
	}

	var portsEqual bool
	if expectedSocket.Port == 0 {
		portsEqual = gotSocket.Port >= EphemeralPortMin && gotSocket.Port <= EphemeralPortMax
	} else {
		portsEqual = expectedSocket.Port == gotSocket.Port
	}

	isEqual = portsEqual &&
		expectedSocket.Zone == gotSocket.Zone &&
		expectedSocket.IP.Equal(gotSocket.IP)
	return
}
