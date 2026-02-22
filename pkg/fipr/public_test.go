package fipr

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestPublic_Full(t *testing.T) {
	tests := []struct {
		name              string
		hmac              string
		shardRunning      bool
		shardDraining     bool
		msgExists         bool
		messageID         string
		onbehalfOf        string
		fragment          string
		clientNoSendFrag  bool
		acceptFragment    bool
		expectedClientErr error
		expectedServerErr error
	}{
		{
			name:           "Successful New Flow",
			hmac:           strings.Repeat("x", HMACSize),
			shardRunning:   true,
			msgExists:      false,
			messageID:      "msg123",
			onbehalfOf:     "127.0.0.1",
			fragment:       "test message fragment",
			acceptFragment: true,
		},
		{
			name:           "Successful Existing Flow",
			hmac:           strings.Repeat("x", HMACSize),
			shardDraining:  true,
			msgExists:      true,
			messageID:      "msg123",
			onbehalfOf:     "127.0.0.1",
			fragment:       "test message fragment",
			acceptFragment: true,
		},
		{
			name:              "Fail New Fragment to Draining",
			hmac:              strings.Repeat("x", HMACSize),
			shardDraining:     true,
			msgExists:         false,
			messageID:         "msg123",
			onbehalfOf:        "127.0.0.1",
			fragment:          "test message fragment",
			acceptFragment:    false,
			expectedClientErr: ErrRemoteRejected,
		},
		{
			name:              "Fail No Message ID",
			hmac:              strings.Repeat("x", HMACSize),
			shardRunning:      true,
			msgExists:         false,
			messageID:         "",
			onbehalfOf:        "127.0.0.1",
			fragment:          "test message fragment",
			expectedClientErr: ErrFrameHasNoPayload,
			expectedServerErr: os.ErrDeadlineExceeded,
		},
		{
			name:              "Client Closes Before Fragment",
			hmac:              strings.Repeat("x", HMACSize),
			shardDraining:     true,
			shardRunning:      true,
			msgExists:         false,
			messageID:         "msg123",
			onbehalfOf:        "127.0.0.1",
			fragment:          "", // no fragment sent
			acceptFragment:    false,
			clientNoSendFrag:  true,
			expectedClientErr: nil,              // client closes cleanly, no error
			expectedServerErr: io.ErrClosedPipe, // server sees EOF/closed waiting for fragment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to bytes for test
			secret := []byte(tt.hmac)
			testFragment := []byte(tt.fragment)

			client, server := net.Pipe()
			clientSession, err := New(client, secret)
			if err != nil {
				t.Fatalf("client session creation failed: %v", err)
			}
			serverSession, err := New(server, secret)
			if err != nil {
				t.Fatalf("server session creation failed: %v", err)
			}

			var wg sync.WaitGroup

			clientErrChan := make(chan error, 6) // Buffered to avoid blocking test run
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := clientSession.SendStart(tt.messageID)
				if err != nil {
					clientErrChan <- fmt.Errorf("failed to send start: %w", err)
					return
				}

				err = clientSession.SendOnBehalfOf(tt.onbehalfOf)
				if err != nil {
					clientErrChan <- fmt.Errorf("failed sending obo: %w", err)
					return
				}
				draining, err := clientSession.SendShardCheck()
				if err != nil {
					clientErrChan <- fmt.Errorf("failed sending shard check: %w", err)
					return
				}
				msgExists, err := clientSession.SendMessageCheck()
				if err != nil {
					clientErrChan <- fmt.Errorf("failed sending message check: %w", err)
					return
				}

				if tt.clientNoSendFrag {
					// Simulates client refusing to route a fragment
					client.Close() // Would happen in the caller
					return
				}

				// Shard is only accepting existing messages (only when unexpected for the test)
				if !msgExists && tt.msgExists && draining && !tt.shardDraining {
					clientErrChan <- fmt.Errorf("remote shard is draining: cannot send new fragments")
					return
				}

				// Shard is accepting new or existing messages
				err = clientSession.SendFragment(testFragment)
				if err != nil {
					clientErrChan <- fmt.Errorf("failed sending fragment: %w", err)
					return
				}
			}()

			serverErrChan := make(chan error, 12) // Buffered to avoid blocking test run
			wg.Add(1)
			go func() {
				defer wg.Done()
				err = serverSession.WaitStart()
				if err != nil {
					serverErrChan <- fmt.Errorf("failed to wait for start: %w", err)
					return
				}

				if serverSession.state != stateStarted {
					serverErrChan <- fmt.Errorf("expected server session tp be started, but it is not started")
				}
				if serverSession.MessageID() != tt.messageID {
					serverErrChan <- fmt.Errorf("expected message ID to be '%s', but got '%s'", tt.messageID, serverSession.MessageID())
				}

				err = serverSession.WaitOnBehalfOf()
				if err != nil {
					serverErrChan <- fmt.Errorf("error waiting for original sender address: %w", err)
					return
				}

				if serverSession.OriginalSender() != tt.onbehalfOf {
					serverErrChan <- fmt.Errorf("expected original sender to be '%s', but got '%s'", tt.onbehalfOf, serverSession.OriginalSender())
				}

				err = serverSession.WaitShardCheck()
				if err != nil {
					serverErrChan <- fmt.Errorf("error waiting for shard check: %w", err)
					return
				}

				err = serverSession.SendShardStatus(tt.shardRunning, tt.shardDraining)
				if err != nil {
					serverErrChan <- fmt.Errorf("error sending shard status: %w", err)
					return
				}

				err = serverSession.WaitMessageCheck()
				if err != nil {
					serverErrChan <- fmt.Errorf("error waiting for message check: %w", err)
					return
				}

				err = serverSession.SendMessageStatus(tt.msgExists)
				if err != nil {
					serverErrChan <- fmt.Errorf("error preparing message status: %w", err)
					return
				}

				rawFragment, err := serverSession.WaitFragment()
				if err != nil {
					serverErrChan <- fmt.Errorf("error waiting for fragment: %w", err)
					return
				}

				if tt.acceptFragment {
					if !bytes.Equal(rawFragment, testFragment) {
						serverErrChan <- fmt.Errorf("expected fragment bytes to be '%x', but got '%x'\n", testFragment, rawFragment)
						err = serverSession.SendReject()
					} else {
						err = serverSession.SendAccept()
					}
				} else {
					err = serverSession.SendReject()
				}
				if err != nil {
					serverErrChan <- fmt.Errorf("failed to send final accept/reject code: %w", err)
				}
			}()
			wg.Wait()

			for len(clientErrChan) > 0 {
				testClientError := <-clientErrChan
				if errors.Is(testClientError, tt.expectedClientErr) {
					continue
				}
				if testClientError != nil {
					t.Errorf("test client had error: %v", testClientError)
				}
			}
			for len(serverErrChan) > 0 {
				testServerError := <-serverErrChan
				if errors.Is(testServerError, tt.expectedServerErr) {
					continue
				}
				if testServerError != nil {
					t.Errorf("test server had error: %v", testServerError)
				}
			}
		})
	}
}
