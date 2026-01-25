package integration

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/receiver"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Tests receiving pipeline using pre-baked packets (startup/shutdown sequence)
func TestRecvConstantFlow(t *testing.T) {
	testDir := t.TempDir()

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
	testIp := findLocalTestIP(ifaces)

	maxPayloadSize, err := network.FindSendingMaxUDPPayload(testIp)
	if err != nil {
		t.Fatalf("failed to find max payload size: %v", err)
	}

	// Mock persistent keys
	priv, pub, err := ecdh.CreatePersistentKey()
	if err != nil {
		t.Fatalf("expected no error from key creation, but got '%v'", err)
	}

	// Mock output
	testOutputsFile := filepath.Join(testDir, "recv-pipeline-test-outputs.txt")

	// Setup logging with in memory
	logVerbosity := 1 // Set to standard for tests
	globalCtx, globalCancel := context.WithCancel(context.Background())
	logger := logctx.NewLogger("global", logVerbosity, globalCtx.Done()) // New logger tied to global
	globalCtx = logctx.WithLogger(globalCtx, logger)                     // Add logger to global ctx

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

	destAddr, err := net.ResolveUDPAddr("udp", testIp+":"+strconv.Itoa(global.DefaultReceiverPort))
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
			packets, err := mockPackets(1, testMessage, maxPayloadSize, pub)
			if err != nil {
				writerErr = err
				return
			}

			// Send test traffic
			for _, packet := range packets {
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

		errors, foundError := filterLogBuffer(globalCtx, "", "Receiver", global.ErrorLog)
		if foundError {
			t.Fatalf("expected no errors from receiver, but got:\n%s\n", errors)
		}

		warnings, foundWarn := filterLogBuffer(globalCtx, "", "Receiver", global.WarnLog)
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
	logger.Wake()
	logger.Wait()

	// Simple check - ensure lines in output file match number of messages sent
	outputContents, err := os.ReadFile(testOutputsFile)
	if err != nil {
		t.Fatalf("expected no error checking output file, but got '%v'", err)
	}
	output := strings.Trim(string(outputContents), "\n")
	lines := strings.Split(output, "\n")
	if output == "" {
		t.Errorf("expected outputs file to have %d lines, but is empty", totalMessagesSent.Load())
	} else if len(lines) != int(totalMessagesSent.Load()) {
		t.Errorf("expected outputs file to have %d lines, but it actually has %d lines", totalMessagesSent.Load(), len(lines))
	}

	// Check for errors post-shutdown
	errors, errorsFound = filterLogBuffer(globalCtx, "", global.NSRecv, global.ErrorLog)
	if errorsFound {
		t.Errorf("expected no errors in receive daemon shutdown, but found:\n%v", errors)
	}
	warns, warnsFound := filterLogBuffer(globalCtx, "", global.NSRecv, global.WarnLog)
	if warnsFound {
		t.Errorf("expected no warnings in receive daemon shutdown, but found:\n%v", warns)
	}
}
