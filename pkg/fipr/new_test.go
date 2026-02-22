package fipr

import (
	"bytes"
	"errors"
	"net"
	"testing"
)

func TestNew(t *testing.T) {
	mockConn1, _ := net.Pipe()

	tests := []struct {
		name        string
		conn        net.Conn
		hmac        []byte
		expectedErr error
	}{
		{
			name: "Normal",
			conn: mockConn1,
			hmac: bytes.Repeat([]byte("x"), HMACSize),
		},
		{
			name:        "Nil connection",
			conn:        nil,
			hmac:        bytes.Repeat([]byte("1"), HMACSize),
			expectedErr: ErrTransportFailure,
		},
		{
			name:        "Invalid HMAC",
			conn:        mockConn1,
			hmac:        bytes.Repeat([]byte("1"), HMACSize+1),
			expectedErr: ErrInvalidHMAC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := New(tt.conn, tt.hmac)
			if err != nil && errors.Is(err, tt.expectedErr) {
				return
			}
			if err != nil && !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error '%v', but got error '%v'", tt.expectedErr, err)
			}
			if !bytes.Equal(session.hmacSecret, tt.hmac) {
				t.Errorf("expected hmac to be '%x', but found in session hmac '%x'", tt.hmac, session.hmacSecret)
			}
		})
	}

}
