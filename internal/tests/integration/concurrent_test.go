package integration

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver"
	"sdsyslog/internal/sender"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests pipelines under high concurrency (multiple senders)
func TestMultipleSenders(t *testing.T) {
	defer func() {
		if fatalError := recover(); fatalError != nil {
			stack := debug.Stack()
			if !strings.Contains(fmt.Sprintf("%v", fatalError), "test timed out after") {
				t.Fatalf("Error: panic in integration test: %v\n%s\n", fatalError, stack)
			}
		}
	}()

	ifaces, err := net.Interfaces() // This requires the golang tool chain have access to network interfaces
	if err != nil {
		t.Fatalf("failed to get network interface list for local system: %v", err)
	}
	testIp := findLocalTestIP(ifaces)

	// Mock persistent keys
	priv, pub, err := ecdh.CreatePersistentKey()
	if err != nil {
		t.Fatalf("expected no error from key creation, but got '%v'", err)
	}

	// Setup logging
	logVerbosity := 1 // Set to standard for tests
	globalCtx, globalCancel := context.WithCancel(context.Background())
	globalCtx = logctx.New(globalCtx, "global", logVerbosity, globalCtx.Done())

	// Mock in/out buffer
	recvOutput := NewPipeBuffer(2 * 1024 * 1024) // 2MB buffer

	// Daemon config (large queues - not testing queue failures here)
	newRecvCfg := receiver.Config{
		ListenIP:               testIp,
		ListenPort:             global.DefaultReceiverPort,
		AutoscaleEnabled:       true,
		AutoscaleCheckInterval: 200 * time.Millisecond,
		MinListeners:           2,
		MinProcessors:          2,
		MinDefrags:             2,
		MinOutputQueueSize:     16 * global.DefaultMinQueueSize,
		MaxOutputQueueSize:     16 * global.DefaultMaxQueueSize,
		ShardBufferSize:        8192,
		MinProcessorQueueSize:  16 * global.DefaultMinQueueSize,
		MaxProcessorQueueSize:  16 * global.DefaultMaxQueueSize,
		RawWriter:              recvOutput,
		MetricCollectionInterval: time.Duration(
			100 * time.Millisecond, // Setting super fast just for test data collection
		),
		MetricMaxAge: 5 * time.Minute,
	}

	// Launch receiver in background
	daemon := receiver.NewDaemon(newRecvCfg)
	err = daemon.Start(globalCtx, priv)
	if err != nil {
		t.Fatalf("expected no receiver startup errors, got error '%v'", err)
	}

	// Wait for startup
	time.Sleep(2 * newRecvCfg.MetricCollectionInterval)

	// Check for any errors in the log buffer
	errorList, errorsFound := filterLogBuffer(globalCtx, "", "", logctx.ErrorLog)
	if errorsFound {
		t.Fatalf("expected no start-up errors in receive pipeline, but found: %v\n", errorList)
	}

	// Test cases
	// Can't get too crazy here - local network kernel buffer becomes bottleneck (drops packets)
	testCases := []struct {
		name           string
		inputText      string
		sendRepeatCtn  int
		senderCount    int
		readTimeout    time.Duration
		packetDeadline time.Duration
	}{
		{
			name:           "Single Short",
			inputText:      "this is a short test message",
			sendRepeatCtn:  1000,
			senderCount:    5,
			readTimeout:    2 * time.Second,
			packetDeadline: 10 * time.Millisecond,
		},
		{
			name:           "Single Max",
			inputText:      strings.Repeat(`{"key":"val1","a":1}`, 61), // 1220 bytes (near to max mtu for 1500 std)
			sendRepeatCtn:  500,
			senderCount:    10,
			readTimeout:    5 * time.Second,
			packetDeadline: 100 * time.Millisecond,
		},
		{
			name:           "Fragmented Bulk",
			inputText:      strings.Repeat(`{"example":true,"text":"y","values":["t","a","b"]}`, 100), // 5000 bytes
			sendRepeatCtn:  20,
			senderCount:    10,
			readTimeout:    20 * time.Second,
			packetDeadline: 500 * time.Millisecond,
		},
	}

	// Run test cases
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			daemon.Mgrs.Assembler.Config.PacketDeadline.Store(tt.packetDeadline.Nanoseconds())

			sendCtx, sendCancel := context.WithCancel(context.Background())
			sendCtx = logctx.New(sendCtx, "senders", logVerbosity, sendCtx.Done())

			var sendDaemons []*sender.Daemon
			var sendInputs []*PipeBuffer
			for range tt.senderCount {
				sendInput := NewPipeBuffer(2 * len(tt.inputText) * tt.sendRepeatCtn)
				sendInputs = append(sendInputs, sendInput)

				// Startup sending daemons
				newSendCfg := sender.Config{
					DestinationIP:         testIp,
					DestinationPort:       global.DefaultReceiverPort,
					AutoscaleEnabled:      true,
					RawInput:              sendInput,
					MinOutputQueueSize:    global.DefaultMinQueueSize,
					MaxOutputQueueSize:    global.DefaultMaxQueueSize,
					MinAssemblerQueueSize: global.DefaultMinQueueSize,
					MaxAssemblerQueueSize: global.DefaultMaxQueueSize,
					MetricCollectionInterval: time.Duration(
						100 * time.Millisecond, // Setting super fast just for test data collection
					),
					MetricMaxAge: 5 * time.Minute,
				}
				senderDaemon := sender.NewDaemon(newSendCfg)
				err = senderDaemon.Start(sendCtx, pub)
				if err != nil {
					t.Fatalf("expected no sender startup errors, got error '%v'", err)
				}
				sendDaemons = append(sendDaemons, senderDaemon)

				// Wait for startup
				time.Sleep(2 * newSendCfg.MetricCollectionInterval)

				// Check for any errors in the log buffer
				errorList, errorsFound = filterLogBuffer(sendCtx, "", "", logctx.ErrorLog)
				if errorsFound {
					t.Fatalf("expected no start-up errors in sending pipeline, but found: %v\n", errorList)
				}
			}

			// Setup writers to send all content at once to each sender daemon
			var ready sync.WaitGroup
			start := make(chan struct{})
			ready.Add(len(sendInputs))
			writeErrors := make(chan error, tt.senderCount)
			for _, inputWriter := range sendInputs {
				go func() {
					// Signal ready
					ready.Done()

					// Wait for start signal
					<-start

					// Write
					for range tt.sendRepeatCtn {
						_, err := inputWriter.Write([]byte(tt.inputText + "\n"))
						if err != nil {
							writeErrors <- fmt.Errorf("expected no error writing to test input, but got '%v'", err)
							return
						}
					}
				}()
			}
			ready.Wait()
			close(start) // Start sender writes

			// Hash input as source of truth for checking output integrity
			expectedOutput := tt.inputText + "\n"

			inputHash, err := hash.MultipleSlices([]byte(expectedOutput))
			if err != nil {
				t.Errorf("expected no error from input hash generation, but got '%v'", err)
			}

			// Retrieve output content
			outputHashes, err := waitForCompleteLines(recvOutput, tt.sendRepeatCtn*tt.senderCount, tt.readTimeout)
			if err != nil {
				t.Errorf("expected no error from reading output buffer, but got '%v'", err)
			}

			if len(writeErrors) > 0 {
				t.Errorf("Input writers encountered errors:\n")
				for len(writeErrors) > 0 {
					err := <-writeErrors
					if err != nil {
						t.Errorf("  %v", err)
					}
				}
			}

			// Confirm message made it through both pipelines and each line is correct
			var totalFailedLines int
			for _, outputHash := range outputHashes {
				if !bytes.Equal(inputHash, outputHash) {
					totalFailedLines++
				}
			}
			if totalFailedLines > 0 {
				t.Errorf("hash of receiver output line does not match input content hash for %d iterations", totalFailedLines)
			}

			for _, daemon := range sendDaemons {
				daemon.Shutdown()
			}

			// Check for errors in input side
			errorList, errorsFound = filterLogBuffer(sendCtx, "", logctx.NSSend, logctx.ErrorLog)
			if errorsFound {
				t.Errorf("expected no errors in sending pipeline, but found: %v\n", errorList)
			}

			// check for errors in the output side
			errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.ErrorLog)
			if errorsFound {
				t.Errorf("expected no errors in receiving pipeline, but found: %v\n", errorList)
			}

			sendCancel()

			// Zero for next test
			recvOutput.Truncate(0)
		})
	}

	// Graceful shutdown
	daemon.Shutdown()

	// Global shutdown
	globalCancel()
	logger := logctx.GetLogger(globalCtx)
	logger.Wake()
	logger.Wait()

	errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in receive daemon shutdown, but found:\n%s", errorList)
	}
	warns, warnsFound := filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in receive daemon shutdown, but found:\n%v", warns)
	}
}
