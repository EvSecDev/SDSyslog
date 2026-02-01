// Integration tests for pipeline components
package integration

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
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
	testDir := t.TempDir()

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

	// Setup logging with in memory
	logVerbosity := 1 // Set to standard for tests
	globalCtx, globalCancel := context.WithCancel(context.Background())
	globalCtx = logctx.New(globalCtx, "global", logVerbosity, globalCtx.Done())

	// Mock output
	testOutputFileName := "integration-test-outputs.txt"
	testOutputsFile := filepath.Join(testDir, testOutputFileName)

	// Daemon config
	newCfg := receiver.Config{
		ListenIP:               testIp,
		ListenPort:             global.DefaultReceiverPort,
		AutoscaleEnabled:       true,
		AutoscaleCheckInterval: 200 * time.Millisecond,
		MinDefrags:             2,
		MinListeners:           2,
		MinProcessors:          2,
		MinOutputQueueSize:     global.DefaultMinQueueSize,
		MaxOutputQueueSize:     global.DefaultMaxQueueSize,
		ShardBufferSize:        1024,
		MinProcessorQueueSize:  global.DefaultMinQueueSize,
		MaxProcessorQueueSize:  global.DefaultMaxQueueSize,
		OutputFilePath:         testOutputsFile,
		MetricCollectionInterval: time.Duration(
			100 * time.Millisecond, // Setting super fast just for test data collection
		),
		MetricMaxAge: 5 * time.Minute,
	}

	// Launch receiver in background
	daemon := receiver.NewDaemon(newCfg)
	err = daemon.Start(globalCtx, priv)
	if err != nil {
		t.Fatalf("expected no receiver startup errors, got error '%v'", err)
	}

	// Wait for startup
	time.Sleep(2 * newCfg.MetricCollectionInterval)

	// Check for any errors in the log buffer
	errors, errorsFound := filterLogBuffer(globalCtx, "", "", global.ErrorLog)
	if errorsFound {
		t.Fatalf("expected no start-up errors in receive pipeline, but found: %v\n", errors)
	}

	// Mock sending source
	testInputFileName := "integration-test-inputs.txt"
	testInputsFile := filepath.Join(testDir, testInputFileName)
	testStateFile := filepath.Join(testDir, "integration-test-input-state.txt")
	testInFile, err := os.OpenFile(testInputsFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		t.Fatalf("failed to open test output file: %v", err)
	}

	// Startup sending daemon
	newSendCfg := sender.Config{
		DestinationIP:         testIp,
		DestinationPort:       global.DefaultReceiverPort,
		AutoscaleEnabled:      true,
		StateFilePath:         testStateFile,
		FileSourcePaths:       []string{testInputsFile},
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
	errors, errorsFound = filterLogBuffer(globalCtx, "", "", global.ErrorLog)
	if errorsFound {
		t.Fatalf("expected no start-up errors in sending pipeline, but found: %v\n", errors)
	}

	// Open test outputs file
	testOutFile, err := os.OpenFile(testOutputsFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0)
	if err != nil {
		t.Fatalf("failed to open test output file: %v", err)
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

	// Run test cases
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			testStartTime := time.Now().Truncate(newSendCfg.MetricCollectionInterval)

			// Write test text to the watched file the desired number of times
			for range tt.sendRepeatCtn {
				_, err := testInFile.WriteString(tt.inputText + "\n")
				if err != nil {
					t.Fatalf("expected no error writing to test input file, but got '%v'", err)
				}
			}

			// Hash input as source of truth for checking output integrity
			expectedNamespace := []string{global.NSSend, global.NSmIngest, global.NSoFile, testInputFileName}
			expectedOutput := tt.inputText + " (" + global.IOCtxKey + "=" + strings.Join(expectedNamespace, "/") + ")" + "\n"

			inputHash, err := hash.MultipleSlices([]byte(expectedOutput))
			if err != nil {
				t.Errorf("expected no error from input hash generation, but got '%v'", err)
			}

			// Retrieve file output content
			outputHashes, err := waitForCompleteLines(testOutFile, tt.sendRepeatCtn)
			if err != nil {
				t.Errorf("expected no error from reading output file, but got '%v'", err)
			}

			// Confirm message made it through both pipelines and each line is correct
			var totalFailedLines int
			for _, outputHash := range outputHashes {
				if !bytes.Equal(inputHash, outputHash) {
					totalFailedLines++
				}
			}
			if totalFailedLines > 0 {
				t.Errorf("hash of receiver output file line does not match input content hash for %d iterations", totalFailedLines)
			}

			// Check for errors in input side
			errors, errorsFound = filterLogBuffer(globalCtx, "", global.NSSend, global.ErrorLog)
			if errorsFound && !tt.expectedSendErr {
				t.Errorf("expected no errors in sending pipeline, but found: %v\n", errors)
			}
			if !errorsFound && tt.expectedSendErr {
				t.Fatalf("expected errors in sending pipeline, but got nil")
			}

			// check for errors in the output side
			errors, errorsFound = filterLogBuffer(globalCtx, "", global.NSRecv, global.ErrorLog)
			if errorsFound && !tt.expectedRecvErr {
				t.Errorf("expected no errors in receiving pipeline, but found: %v\n", errors)
			}
			if !errorsFound && tt.expectedRecvErr {
				t.Fatalf("expected errors in receiving pipeline, but got nil")
			}

			// Check metrics at in/out pipeline boundaries for expected counts
			err = checkPipelineCounts(tt.sendRepeatCtn, testStartTime, senderDaemon, daemon, newSendCfg.MetricCollectionInterval)
			if err != nil {
				t.Fatalf("Metric test error: %v", err)
			}

			// Zero for next test
			err = testOutFile.Truncate(0)
			if err != nil {
				t.Fatalf("expected no error from truncating test output file, but got '%v'", err)
			}
		})
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
	errors, errorsFound = filterLogBuffer(globalCtx, "", global.NSSend, global.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in send daemon shutdown, but found:\n%s", errors)
	}
	warns, warnsFound := filterLogBuffer(globalCtx, "", global.NSSend, global.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in send daemon shutdown, but found:\n%v", warns)
	}
	errors, errorsFound = filterLogBuffer(globalCtx, "", global.NSRecv, global.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in receive daemon shutdown, but found:\n%s", errors)
	}
	warns, warnsFound = filterLogBuffer(globalCtx, "", global.NSRecv, global.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in receive daemon shutdown, but found:\n%v", warns)
	}
}
