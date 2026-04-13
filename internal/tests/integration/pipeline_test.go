// Integration tests for pipeline components
package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"sdsyslog/internal/crypto/hash"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver"
	"sdsyslog/internal/sender"
	"sdsyslog/pkg/crypto/registry"
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
	var cryptoSuite uint8 = 1
	info, validID := registry.GetSuiteInfo(cryptoSuite)
	if !validID {
		t.Fatalf("invalid suite ID %d", cryptoSuite)
	}
	priv, pub, err := info.NewKey()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	// Setup logging
	logVerbosity := 1 // Set to standard for tests
	globalCtx, globalCancel := context.WithCancel(context.Background())
	globalCtx = logctx.New(globalCtx, "global", logVerbosity, globalCtx.Done())

	// Mock in/out buffer
	recvOutput := NewPipeBuffer(10 * 1024 * 1024) // 10MB buffer
	sendInput := NewPipeBuffer(10 * 1024 * 1024)

	// Mocking real config files
	testDir := t.TempDir()
	recvJSONConfFile := filepath.Join(testDir, "sdsyslog.json")
	sendJSONConfFile := filepath.Join(testDir, "sdsyslog-sender.json")
	privKeyFile := filepath.Join(testDir, "private-key")

	err = os.WriteFile(privKeyFile, []byte(base64.StdEncoding.EncodeToString(priv)), 0600)
	if err != nil {
		t.Fatalf("failed to write private key file: %v", err)
	}

	// Daemon config
	newRecvJSONCfg := receiver.JSONConfig{
		PrivateKeyFile: privKeyFile,
		Network: struct {
			Address string "json:\"address\""
			Port    int    "json:\"port\""
		}{
			Address: testIp,
			Port:    global.DefaultReceiverPort,
		},
		Metrics: struct {
			Interval          string "json:\"collectionInterval\""
			MaxAge            string "json:\"maximumRetention,omitempty\""
			EnableQueryServer bool   "json:\"enableHTTPQueryServer\""
			QueryServerPort   int    "json:\"HTTPQueryServerPort\""
		}{
			Interval:          "100ms", // Setting super fast just for test data collection
			MaxAge:            "5m",
			EnableQueryServer: false,
		},
		AutoScaling: struct {
			Enabled          bool            "json:\"enabled\""
			PollInterval     string          "json:\"pollInterval\""
			MinListeners     global.MinValue "json:\"minListeners,omitempty\""
			MaxListeners     global.MaxValue "json:\"maxListeners,omitempty\""
			MinProcessors    global.MinValue "json:\"minProcessors,omitempty\""
			MaxProcessors    global.MaxValue "json:\"maxProcessors,omitempty\""
			MinProcQueueSize global.MinValue "json:\"minProcQueueSize,omitempty\""
			MaxProcQueueSize global.MaxValue "json:\"maxProcQueueSize,omitempty\""
			MinDefrags       global.MinValue "json:\"minAssemblers,omitempty\""
			MaxDefrags       global.MaxValue "json:\"maxAssemblers,omitempty\""
			MinOutQueueSize  global.MinValue "json:\"minOutQueueSize,omitempty\""
			MaxOutQueueSize  global.MaxValue "json:\"maxOutQueueSize,omitempty\""
		}{
			Enabled:       true,
			PollInterval:  "200ms",
			MinListeners:  1,
			MinProcessors: 1,
			MinDefrags:    1,
		},
	}
	rawJSONCfg, err := json.MarshalIndent(newRecvJSONCfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to parse test recv daemon config: %v", err)
	}
	err = os.WriteFile(recvJSONConfFile, rawJSONCfg, 0600)
	if err != nil {
		t.Fatalf("failed to create test recv daemon config file: %v", err)
	}

	recvJsonCfg, err := receiver.LoadConfig(recvJSONConfFile)
	if err != nil {
		t.Fatalf("failed to load test recv daemon config file: %v", err)
	}

	newRecvCfg, err := recvJsonCfg.NewDaemonConf(recvJSONConfFile, false)
	if err != nil {
		t.Fatalf("failed to parse loaded test recv daemon config file: %v", err)
	}

	newRecvCfg.RawWriter = recvOutput // For testing only

	privateKey, err := os.ReadFile(recvJsonCfg.PrivateKeyFile)
	if err != nil {
		t.Fatalf("failed to read receiver private key file: %v", err)
	}

	key, err := base64.StdEncoding.DecodeString(string(privateKey))
	if err != nil {
		t.Fatalf("failed to decode receiver private key: %v", err)
	}

	// Launch receiver in background
	daemon := receiver.NewDaemon(newRecvCfg)
	err = daemon.Start(globalCtx, key)
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
	newSendJSONCfg := sender.JSONConfig{
		PublicKey: base64.StdEncoding.EncodeToString(pub),
		Network: struct {
			Address        string "json:\"address\""
			Port           int    "json:\"port\""
			MaxPayloadSize int    "json:\"maxPayloadSize,omitempty\""
		}{
			Address: testIp,
			Port:    global.DefaultReceiverPort,
		},
		Metrics: struct {
			Interval          string "json:\"collectionInterval\""
			MaxAge            string "json:\"maximumRetention,omitempty\""
			EnableQueryServer bool   "json:\"enableHTTPQueryServer\""
			QueryServerPort   int    "json:\"HTTPQueryServerPort\""
		}{
			Interval:          "100ms",
			MaxAge:            "5m",
			EnableQueryServer: false,
		},
		AutoScaling: struct {
			Enabled               bool            "json:\"enabled\""
			PollInterval          string          "json:\"pollInterval\""
			MinOutputs            global.MinValue "json:\"minOutputs,omitempty\""
			MaxOutputs            global.MaxValue "json:\"maxOutputs,omitempty\""
			MinAssemblers         global.MinValue "json:\"minAssemblers,omitempty\""
			MaxAssemblers         global.MaxValue "json:\"maxAssemblers,omitempty\""
			MinOutputQueueSize    global.MinValue "json:\"minOutputQueueSize,omitempty\""
			MaxOutputQueueSize    global.MaxValue "json:\"maxOutputQueueSize,omitempty\""
			MinAssemblerQueueSize global.MinValue "json:\"minAssemblerQueueSize,omitempty\""
			MaxAssemblerQueueSize global.MaxValue "json:\"maxAssemblerQueueSize,omitempty\""
		}{
			Enabled:       true,
			PollInterval:  "200ms",
			MinOutputs:    2,
			MinAssemblers: 2,
		},
	}
	rawJSONCfg, err = json.MarshalIndent(newSendJSONCfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to parse test send daemon config: %v", err)
	}
	err = os.WriteFile(sendJSONConfFile, rawJSONCfg, 0600)
	if err != nil {
		t.Fatalf("failed to create test send daemon config file: %v", err)
	}

	sendJsonCfg, err := sender.LoadConfig(sendJSONConfFile)
	if err != nil {
		t.Fatalf("failed to load test send daemon config file: %v", err)
	}

	newSendCfg, err := sendJsonCfg.NewDaemonConf(sendJSONConfFile, false)
	if err != nil {
		t.Fatalf("failed to parse loaded test send daemon config file: %v", err)
	}

	newSendCfg.RawInput = sendInput // For testing only

	publicKey, err := base64.StdEncoding.DecodeString(sendJsonCfg.PublicKey)
	if err != nil {
		t.Fatalf("failed to decode sender public key: %v", err)
	}

	senderDaemon := sender.NewDaemon(newSendCfg)
	err = senderDaemon.Start(globalCtx, publicKey)
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
