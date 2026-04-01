// Integration tests for pipeline components
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
	"testing"
	"time"
)

// Tests full sending and receiving pipelines including daemon startup/shutdown
func TestSendReceivePipeline(t *testing.T) {
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
	recvOutput := NewPipeBuffer(10 * 1024 * 1024) // 10MB buffer
	sendInput := NewPipeBuffer(10 * 1024 * 1024)

	// Daemon config
	newRecvCfg := receiver.Config{
		ListenIP:               testIp,
		ListenPort:             global.DefaultReceiverPort,
		AutoscaleEnabled:       true,
		AutoscaleCheckInterval: 200 * time.Millisecond,
		MinDefrags:             1,
		MinListeners:           1,
		MinProcessors:          1,
		MinOutputQueueSize:     global.DefaultMinQueueSize,
		MaxOutputQueueSize:     global.DefaultMaxQueueSize,
		ShardBufferSize:        1024,
		MinProcessorQueueSize:  global.DefaultMinQueueSize,
		MaxProcessorQueueSize:  global.DefaultMaxQueueSize,
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

	// Startup sending daemon
	newSendCfg := sender.Config{
		DestinationIP:         testIp,
		DestinationPort:       global.DefaultReceiverPort,
		AutoscaleEnabled:      true,
		RawInput:              sendInput,
		MinOutputs:            2,
		MinAssemblers:         2,
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
	err = senderDaemon.Start(globalCtx, pub)
	if err != nil {
		t.Fatalf("expected no sender startup errors, got error '%v'", err)
	}

	// Wait for startup
	time.Sleep(2 * newSendCfg.MetricCollectionInterval)

	// Check for any errors in the log buffer
	errorList, errorsFound = filterLogBuffer(globalCtx, "", "", logctx.ErrorLog)
	if errorsFound {
		t.Fatalf("expected no start-up errors in sending pipeline, but found: %v\n", errorList)
	}

	// Test cases
	testCases := []struct {
		name            string
		inputText       string
		sendRepeatCtn   int
		expectedSendErr bool
		expectedRecvErr bool
	}{
		{
			name:          "Single One",
			inputText:     "a",
			sendRepeatCtn: 1,
		},
		{
			name:          "Single Short",
			inputText:     "this is a short test message",
			sendRepeatCtn: 10,
		},
		{
			name:          "Single Max",
			inputText:     strings.Repeat(`{"key":"val1","a":1}`, 61), // 1220 bytes (near to max mtu for 1500 std)
			sendRepeatCtn: 10,
		},
		{
			name:          "Fragmented Long",
			inputText:     strings.Repeat(`{"i":1,"n":"ice","state":true}`, 100), // 3000 bytes
			sendRepeatCtn: 100,
		},
		{
			name:          "Fragmented Bulk",
			inputText:     strings.Repeat(`{"example":true,"text":"y","values":["t","a","b"]}`, 100), // 5000 bytes
			sendRepeatCtn: 1000,
		},
		{
			name:          "Excessive Message Size",
			inputText:     strings.Repeat(`{"example":true,"text":"y","values":["t","a","b"]}`, 1000), // 50000 bytes
			sendRepeatCtn: 100,
		},
	}

	// For metric check
	var totalSent int
	testsStartTime := time.Now().Truncate(newSendCfg.MetricCollectionInterval)

	// Run test cases
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Write test text to the watched buffer the desired number of times
			for range tt.sendRepeatCtn {
				_, err := sendInput.Write([]byte(tt.inputText + "\n"))
				if err != nil {
					t.Fatalf("expected no error writing to test input, but got '%v'", err)
				}
			}
			totalSent += tt.sendRepeatCtn

			// Hash input as source of truth for checking output integrity
			expectedOutput := tt.inputText + "\n"

			inputHash, err := hash.MultipleSlices([]byte(expectedOutput))
			if err != nil {
				t.Errorf("expected no error from input hash generation, but got '%v'", err)
			}

			// Retrieve output content
			outputHashes, err := waitForCompleteLines(recvOutput, tt.sendRepeatCtn, 10*time.Second)
			if err != nil {
				t.Errorf("expected no error from reading output buffer, but got '%v'", err)
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

			// Check for errors in input side
			errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSSend, logctx.ErrorLog)
			if errorsFound && !tt.expectedSendErr {
				t.Errorf("expected no errors in sending pipeline, but found: %v\n", errorList)
			}
			if !errorsFound && tt.expectedSendErr {
				t.Fatalf("expected errors in sending pipeline, but got nil")
			}

			// check for errors in the output side
			errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.ErrorLog)
			if errorsFound && !tt.expectedRecvErr {
				t.Errorf("expected no errors in receiving pipeline, but found: %v\n", errorList)
			}
			if !errorsFound && tt.expectedRecvErr {
				t.Fatalf("expected errors in receiving pipeline, but got nil")
			}

			// Zero for next test
			recvOutput.Truncate(0)
			sendInput.Truncate(0)
		})
	}

	// Check metrics at in/out pipeline boundaries for expected counts
	// Checks during test can be unreliable due to timing and metric bucket slices
	// Check once after tests are done for total expected count
	err = checkPipelineCounts(totalSent, testsStartTime, senderDaemon, daemon, newSendCfg.MetricCollectionInterval)
	if err != nil {
		t.Errorf("Metric test error: %v", err)
	}

	// Graceful shutdown
	senderDaemon.Shutdown()
	daemon.Shutdown()

	// Global shutdown
	globalCancel()
	logger := logctx.GetLogger(globalCtx)
	logger.Wake()
	logger.Wait()

	// Check for errors post-shutdown
	errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSSend, logctx.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in send daemon shutdown, but found:\n%s", errorList)
	}
	warns, warnsFound := filterLogBuffer(globalCtx, "", logctx.NSSend, logctx.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in send daemon shutdown, but found:\n%v", warns)
	}
	errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in receive daemon shutdown, but found:\n%s", errorList)
	}
	warns, warnsFound = filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in receive daemon shutdown, but found:\n%v", warns)
	}
}
