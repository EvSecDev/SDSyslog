package fipr

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcknowledgements(t *testing.T) {
	client, server := net.Pipe()
	hmacSecret := bytes.Repeat([]byte("a"), HMACSize)
	clientSession, err := New(client, hmacSecret)
	if err != nil {
		t.Fatalf("failed to create test client session: %v", err)
	}
	serverSession, err := New(server, hmacSecret)
	if err != nil {
		t.Fatalf("failed to create test server session: %v", err)
	}

	ack := uint16(1)

	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errChan <- clientSession.awaitAck(ack)
	}()

	err = serverSession.sendAck(ack)
	if err != nil {
		t.Fatalf("failed to setup session: step: wait start: %v", err)
	}
	wg.Wait()

	sendError := <-errChan
	if sendError != nil {
		t.Fatalf("failed to setup session: step: start: %v", sendError)
	}

}

func TestResend(t *testing.T) {
	clientRaw, server := net.Pipe()
	hmacSecret := bytes.Repeat([]byte("a"), HMACSize)
	client := newMockConn(clientRaw, hmacSecret) // Creates invalid payloads once
	clientSession, err := New(client, hmacSecret)
	if err != nil {
		t.Fatalf("failed to create test client session: %v", err)
	}
	serverSession, err := New(server, hmacSecret)
	if err != nil {
		t.Fatalf("failed to create test server session: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := clientSession.send(opShardStatus, []byte{byte(shardShutdown)})
		errChan <- err
	}()

	response, err := serverSession.await(opShardStatus)
	if err != nil {
		t.Errorf("failed awaiting status: %v", err)
	}

	wg.Wait()
	for len(errChan) > 0 {
		testClientError := <-errChan
		if testClientError != nil {
			t.Fatalf("test client had error: %v", testClientError)
		}
	}

	// Verify resent status has correct status
	if response.payload[0] != byte(shardShutdown) {
		t.Errorf("expected final response payload for shard status to be '%x', but got '%x'", shardShutdown, response.payload[0])
	}

	// Verify server session reflects resend (0=original, 1=resend, 2=freshframe, 3=ack, 4=nextseq)
	if serverSession.seq != 4 {
		t.Errorf("expected next session sequence to be 4, but got %d", serverSession.seq)
	}
}

func TestResend_MaxRetries(t *testing.T) {
	client, server := net.Pipe()
	hmacSecret := bytes.Repeat([]byte("a"), HMACSize)
	clientSession, err := New(client, hmacSecret)
	if err != nil {
		t.Fatalf("failed to create test client session: %v", err)
	}
	serverSession, err := New(server, hmacSecret)
	if err != nil {
		t.Fatalf("failed to create test server session: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := clientSession.send(opShardStatus, []byte{}) // Invalid payload always
		errChan <- err
	}()

	// Wrap await in timeout
	var awaitErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, awaitErr = serverSession.await(opShardStatus)
	}()

	timeout := (maxWaitTimeForFrame + maxWaitTimeForSend) * time.Duration(maxConsecutiveResends)
	select {
	case <-done:
		// await finished
	case <-time.After(timeout):
		t.Fatal("max retries never hit, test timed out")
	}

	if !errors.Is(awaitErr, ErrTooManyResends) {
		t.Fatalf("expected error %v, but got error %v", ErrTooManyResends, err)
	}

	server.Close() // Simulate server closing connection after failing to wait for valid data
	wg.Wait()

	for len(errChan) > 0 {
		testClientError := <-errChan
		if testClientError != nil && errors.Is(err, ErrTransportWasClosed) {
			t.Fatalf("test client had error: %v", testClientError)
		}
	}
}

type mockConn struct {
	net.Conn
	hmacSecret  []byte
	corruptNext atomic.Bool
}

func newMockConn(inner net.Conn, secret []byte) *mockConn {
	c := &mockConn{
		Conn:       inner,
		hmacSecret: secret,
	}
	c.corruptNext.Store(true)
	return c
}
func (c *mockConn) Write(b []byte) (n int, err error) {
	if !c.corruptNext.Load() {
		return c.Conn.Write(b)
	}

	if len(b) < minDataLength {
		return c.Conn.Write(b)
	}

	c.corruptNext.Store(false)

	// Since we are mutating, we want the writer to think they wrote exactly their payload
	n = len(b)

	// Happens once
	tempSession, err := New(c.Conn, c.hmacSecret) // Only need for encode/decode easy access, discarded after this
	if err != nil {
		err = fmt.Errorf("TEST-WIREMUTATOR: failed to make temp session: %w", err)
		return
	}
	framebody, err := tempSession.decodeFrame(b)
	if err != nil {
		err = fmt.Errorf("TEST-WIREMUTATOR: failed to decode real frame: %w", err)
		return
	}
	tempSession.seq = 0 // mutate back for reencode to mock the original frame
	fakeFrame, err := tempSession.encodeFrame(framebody.op, nil)
	if err != nil {
		err = fmt.Errorf("TEST-WIREMUTATOR: failed to encode new fake frame: %w", err)
		return
	}

	_, err = c.Conn.Write(fakeFrame) // Now send on the mutated frame
	return
}
