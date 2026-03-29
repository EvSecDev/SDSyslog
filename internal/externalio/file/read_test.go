package file

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/filtering"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/queue/mpmc"
	"sdsyslog/pkg/protocol"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReader(t *testing.T) {
	localHostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("unexpected error retrieving system hostname: %v", err)
	}

	tests := []struct {
		name                 string
		inputLines           []string
		filters              []protocol.MessageFilter
		rotationIndex        int // Rotate at this input line slice index
		rotationFunc         func(filePath string, file *os.File) (newFile *os.File, err error)
		expectedMsgs         []protocol.Message
		expectedErrorMessage string
		expectedLinesRead    uint64
		expectedProcCount    uint64
	}{
		{
			name:       "basic single line",
			inputLines: []string{"test message"},
			expectedMsgs: []protocol.Message{
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message"),
				},
			},
			expectedLinesRead: 1,
			expectedProcCount: 1,
		},
		{
			name:       "long single line",
			inputLines: []string{strings.Repeat("a", 65537)},
			expectedMsgs: []protocol.Message{
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte(strings.Repeat("a", 65537)),
				},
			},
			expectedLinesRead: 1,
			expectedProcCount: 1,
		},
		{
			name: "multiple lines",
			inputLines: []string{
				"test message 01",
				"test message 02",
				"test message 03",
			},
			expectedMsgs: []protocol.Message{
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 01"),
				},
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 02"),
				},
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 03"),
				},
			},
			expectedLinesRead: 3,
			expectedProcCount: 3,
		},
		{
			name: "multiple with filter",
			inputLines: []string{
				"test message 01",
				"invalid message",
				"test message 03",
			},
			filters: []protocol.MessageFilter{
				{
					Data: &filtering.Filter{
						Exact: "invalid message",
					},
				},
			},
			expectedMsgs: []protocol.Message{
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 01"),
				},
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 03"),
				},
			},
			expectedLinesRead: 3,
			expectedProcCount: 2,
		},
		{
			name: "copy create log rotation",
			inputLines: []string{
				"test message 01",
				"test message 02",
				"test message 03",
			},
			rotationIndex: 1,
			rotationFunc: func(filePath string, file *os.File) (newFile *os.File, err error) {
				_ = file
				err = os.Rename(filePath, filePath+".1")
				if err != nil {
					err = fmt.Errorf("failed to rename log file: %v", err)
					return
				}
				newFile, err = os.Create(filePath)
				if err != nil {
					err = fmt.Errorf("failed to create new log file: %v", err)
					return
				}
				return
			},
			expectedMsgs: []protocol.Message{
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 01"),
				},
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 02"),
				},
				{
					Timestamp: time.Now(),
					Hostname:  localHostname,
					Data:      []byte("test message 03"),
				},
			},
			expectedLinesRead: 3,
			expectedProcCount: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Per test mocks
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = logctx.New(ctx, logctx.NSTest, 1, ctx.Done())

			tempDir := t.TempDir()
			logFilePath := filepath.Join(tempDir, "log")
			stateFile := filepath.Join(tempDir, "state")

			logFile, err := os.Create(logFilePath)
			if err != nil {
				t.Fatalf("failed to create log file: %v", err)
			}
			err = logFile.Close()
			if err != nil {
				t.Fatalf("failed to close log file: %v", err)
			}
			defer func() {
				_ = os.Remove(logFilePath)
				_ = os.Remove(stateFile)
			}()

			queue, err := mpmc.New[protocol.Message]([]string{logctx.NSTest}, 1024, global.MinValue(1024), global.MaxValue(1024))
			if err != nil {
				t.Fatalf("unexpected error creating queue: %v", err)
			}
			inMod, err := NewInput(ctx, logFilePath, stateFile, tt.filters, queue)
			if err != nil {
				t.Fatalf("unexpected error creating input module: %v", err)
			}

			var readerWait sync.WaitGroup
			readerWait.Add(1)
			go func() {
				defer readerWait.Done()
				inMod.Reader(ctx)
			}()

			// Send input to the watched file
			logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_RDWR, 06440)
			if err != nil {
				t.Fatalf("failed to open log file: %v", err)
			}
			for index, inputLine := range tt.inputLines {
				if tt.rotationFunc != nil && tt.rotationIndex == index {
					logFile, err = tt.rotationFunc(logFilePath, logFile)
					if err != nil {
						t.Fatalf("failed test rotation: %v", err)
					}
				}
				_, err = fmt.Fprintf(logFile, "%s\n", inputLine)
				if err != nil {
					t.Fatalf("failed to write to log file: %v", err)
				}
			}
			err = logFile.Close()
			if err != nil {
				t.Fatalf("failed to close log file after reading: %v", err)
			}

			// Check outputs in queue (with timeout)
			outputs := make(chan protocol.Message, len(tt.expectedMsgs))
			errChan := make(chan error, len(tt.expectedMsgs))
			done := make(chan struct{})
			go func() {
				for range len(tt.expectedMsgs) {
					msg, success := queue.Pop(ctx)
					if !success {
						errChan <- fmt.Errorf("failed to pop message from queue")
					} else {
						outputs <- msg
					}
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Errorf("queue pop timed out (most likely reader did not produce the expected number of output messages)")
			}
			if len(errChan) > 0 {
				for range errChan {
					err := <-errChan
					t.Errorf("%v", err)
				}
			}

			// Shutdown instance
			cancel()
			readerWait.Wait()
			err = inMod.Shutdown()
			if err != nil {
				t.Fatalf("failed to shutdown input module: %v", err)
			}

			// Clean unneeded files now
			err = os.Remove(logFilePath)
			if err != nil && !os.IsNotExist(err) {
				t.Errorf("failed to remove log file %q: %v", logFilePath, err)
			}
			err = os.Remove(stateFile)
			if err != nil && !os.IsNotExist(err) {
				t.Errorf("failed to remove state file %q: %v", stateFile, err)
			}
			err = os.Remove(logFilePath + ".1")
			if err != nil && !os.IsNotExist(err) {
				t.Errorf("failed to remove rotated log file %q: %v", logFilePath+".1", err)
			}

			// Check message output validity
			if len(outputs) == 0 {
				t.Errorf("no outputs from queue received")
			} else if len(outputs) != len(tt.expectedMsgs) {
				t.Errorf("expected %d outputs, but got %d outputs:", len(tt.expectedMsgs), len(outputs))
				for i := 0; len(outputs) > 0; i++ {
					msg := <-outputs
					t.Errorf("got output: %+v", msg)
				}
				for _, expected := range tt.expectedMsgs {
					t.Errorf("expected:   %+v", expected)
				}
			} else {
				for i := 0; len(outputs) > 0; i++ {
					msg := <-outputs
					if tt.expectedMsgs[i].Hostname != msg.Hostname {
						t.Errorf("msg: expected hostname %q, but got %q", tt.expectedMsgs[i].Hostname, msg.Hostname)
					}
					if !bytes.Equal(tt.expectedMsgs[i].Data, msg.Data) {
						t.Errorf("msg: expected data %q, but got %q", string(tt.expectedMsgs[i].Data), string(msg.Data))
					}
				}
			}

			// Collect metrics after worker is fully shutdown
			metrics := inMod.CollectMetrics(1 * time.Second)

			// Validate no errors in log
			logger := logctx.GetLogger(ctx)
			lines := logger.GetFormattedLogLines()
			var foundErrors []string
			var foundExpectedError bool
			for _, line := range lines {
				if strings.Contains(line, "["+logctx.InfoLog+"]") {
					continue
				}
				if tt.expectedErrorMessage != "" && strings.Contains(line, tt.expectedErrorMessage) {
					foundExpectedError = true
					continue // Search for other errors
				}
				foundErrors = append(foundErrors, line)
			}
			if tt.expectedErrorMessage != "" && !foundExpectedError {
				t.Errorf("expected error %q to be in the log buffer but found nothing", tt.expectedErrorMessage)
			}
			if len(foundErrors) > 0 {
				t.Errorf("expected no errors in log buffer, but found lines:\n")
				for _, err := range foundErrors {
					t.Errorf("%s", err)
				}
			}

			// Validate metrics from the collection func point of view
			for _, metric := range metrics {
				value := metric.Value.Raw.(uint64)
				if metric.Name == MTLinesRead && value != tt.expectedLinesRead {
					t.Errorf("expected metric lines read count to be %d, but got %d", tt.expectedLinesRead, value)
				}
				if metric.Name == MTSuc && value != tt.expectedProcCount {
					t.Errorf("expected metric success process count to be %d, but got %d", tt.expectedProcCount, value)
				}
			}
		})
	}
}
