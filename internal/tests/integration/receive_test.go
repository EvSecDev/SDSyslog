package integration

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/receiver"
	"sdsyslog/pkg/crypto/registry"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Tests receiving pipeline using pre-baked packets (startup/shutdown sequence)
func TestRecvConstantFlow(t *testing.T) {
	defer func() {
		if fatalError := recover(); fatalError != nil {
			stack := debug.Stack()
			t.Fatalf("Error: panic in receiver pipeline test: %v\n%s\n", fatalError, stack)
		}
	}()

	ifaces, err := net.Interfaces() // This requires the golang tool chain have access to network interfaces
	if err != nil {
		t.Fatalf("failed to get network interface list for local system: %v", err)
	}
	testIP := findLocalTestIP(ifaces)

	maxPayloadSize, err := network.FindSendingMaxUDPPayload(testIP)
	if err != nil {
		t.Fatalf("failed to find max payload size: %v", err)
	}

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

	// Mock output
	recvOutput := NewPipeBuffer(10 * 1024 * 1024) // 10MB buffer

	// Setup logging with in memory
	logVerbosity := 1 // Set to standard for tests
	globalCtx, globalCancel := context.WithCancel(context.Background())
	globalCtx = logctx.New(globalCtx, "global", logVerbosity, globalCtx.Done())

	// Mocking real config files
	testDir := t.TempDir()

	// Daemon config
	newRecvJSONCfg := receiver.JSONOptions{
		Network: struct {
			Address string "json:\"address\""
			Port    int    "json:\"port\""
		}{
			Address: testIP,
			Port:    global.DefaultReceiverPort,
		},
		Metrics: struct {
			Interval          parsing.Duration "json:\"collectionInterval\""
			MaxAge            parsing.Duration "json:\"maximumRetention,omitempty\""
			EnableQueryServer bool             "json:\"enableHTTPQueryServer\""
			QueryServerPort   int              "json:\"HTTPQueryServerPort\""
		}{
			Interval:          parsing.Duration(100 * time.Millisecond), // Setting super fast just for test data collection
			MaxAge:            parsing.Duration(5 * time.Minute),
			EnableQueryServer: false,
		},
		AutoScaling: struct {
			Enabled          bool             "json:\"enabled\""
			PollInterval     parsing.Duration "json:\"pollInterval\""
			MinListeners     global.MinValue  "json:\"minListeners,omitempty\""
			MaxListeners     global.MaxValue  "json:\"maxListeners,omitempty\""
			MinProcessors    global.MinValue  "json:\"minProcessors,omitempty\""
			MaxProcessors    global.MaxValue  "json:\"maxProcessors,omitempty\""
			MinProcQueueSize global.MinValue  "json:\"minProcQueueSize,omitempty\""
			MaxProcQueueSize global.MaxValue  "json:\"maxProcQueueSize,omitempty\""
			MinDefrags       global.MinValue  "json:\"minAssemblers,omitempty\""
			MaxDefrags       global.MaxValue  "json:\"maxAssemblers,omitempty\""
			MinOutQueueSize  global.MinValue  "json:\"minOutQueueSize,omitempty\""
			MaxOutQueueSize  global.MaxValue  "json:\"maxOutQueueSize,omitempty\""
		}{
			Enabled:       true,
			PollInterval:  parsing.Duration(2 * global.DefaultMinPacketDeadline),
			MinListeners:  2,
			MinProcessors: 2,
			MinDefrags:    2,
		},
	}
	daemon, err := setupRecvDaemon(globalCtx, newRecvJSONCfg, testDir, priv, recvOutput)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Launch receiver in background
	err = daemon.Start()
	if err != nil {
		t.Fatalf("expected no receiver startup errors, got error '%v'", err)
	}

	// Wait for startup
	time.Sleep(2 * time.Duration(newRecvJSONCfg.Metrics.Interval))

	// Check for any errors in the log buffer
	errorList, errorsFound := filterLogBuffer(globalCtx, "", "", logctx.ErrorLog)
	if errorsFound {
		t.Fatalf("expected no start-up errors in receive pipeline, but found: %v\n", errorList)
	}

	destAddr, err := net.ResolveUDPAddr("udp", testIP+":"+strconv.Itoa(global.DefaultReceiverPort))
	if err != nil {
		t.Fatalf("failed to resolve destination: %v", err)
	}

	testMessage, err := mockMessage(`{"i":1,"n":"ice","state":true}`, 100) // more than 1 packet per msg
	if err != nil {
		t.Fatalf("expected no error in creating mock message, but got '%v'", err)
	}

	// Setup connection to receiver
	destConn, err := net.DialUDP("udp", nil, destAddr)
	if err != nil {
		t.Fatalf("failed to open udp socket: %v", err)
	}

	var totalMessagesSent atomic.Uint64

	// Continuous packet writer
	writerCtx, writerCancel := context.WithCancel(context.Background())
	var writerWaiter sync.WaitGroup
	writerWaiter.Add(1)
	var writerErr error
	go func() {
		defer writerWaiter.Done()

		for {
			select {
			case <-writerCtx.Done():
				return
			default:
			}

			// Get mock packets - new set (for random log id)
			packets, err := mockPackets(1, []byte(testMessage), maxPayloadSize, pub)
			if err != nil {
				writerErr = err
				return
			}

			// Send test traffic
			for _, packet := range packets {
				if len(packet) > maxPayloadSize {
					writerErr = fmt.Errorf("expected maximum payload size to create packets of size %d, but got packet of size %d",
						maxPayloadSize, len(packet))
				}

				_, err := destConn.Write(packet)
				if err != nil {
					writerErr = err
					return
				}
			}
			totalMessagesSent.Add(1)
		}
	}()
	if writerErr != nil {
		t.Fatalf("encountered error while writing mock packets: %v", err)
	}

	// Pipeline watcher
	startWatchTime := time.Now()
	maxWatchTime := 5 * time.Second
	pollingInterval := 500 * time.Millisecond
	for {
		if time.Since(startWatchTime) > maxWatchTime {
			writerCancel()
			break
		}

		errorList, foundError := filterLogBuffer(globalCtx, "", "Receiver", logctx.ErrorLog)
		if foundError {
			t.Fatalf("expected no errors from receiver, but got:\n%s\n", errorList)
		}

		warnings, foundWarn := filterLogBuffer(globalCtx, "", "Receiver", logctx.WarnLog)
		if foundWarn {
			t.Errorf("expected no warnings from receiver, but got:\n%s\n", warnings)
		}

		time.Sleep(pollingInterval)
	}
	writerWaiter.Wait()

	// Graceful shutdown
	daemon.Shutdown()

	// Global shutdown
	globalCancel()
	logger := logctx.GetLogger(globalCtx)
	logger.Wake()
	logger.Wait()

	// Simple check - ensure lines in output file match number of messages sent

	output := strings.Trim(string(recvOutput.buffer), "\n")
	lines := strings.Split(output, "\n")
	if output == "" {
		t.Errorf("expected output buffer to have %d lines, but is empty", totalMessagesSent.Load())
	} else if len(lines) != int(totalMessagesSent.Load()) {
		t.Errorf("expected output buffer to have %d lines, but it actually has %d lines", totalMessagesSent.Load(), len(lines))
	}

	// Check for errors post-shutdown
	errorList, errorsFound = filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in receive daemon shutdown, but found:\n%s", errorList)
	}
	warns, warnsFound := filterLogBuffer(globalCtx, "", logctx.NSRecv, logctx.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in receive daemon shutdown, but found:\n%s", warns)
	}
}
