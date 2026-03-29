package file

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"strings"
	"testing"
	"time"
)

func TestWriter(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name                 string
		inputs               []protocol.Payload
		batchSize            int
		expectedWriteCount   int
		expectedErrorMessage string
	}{
		{
			name: "basic single line",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      []byte("test message"),
				},
			},
			expectedWriteCount: 1,
		},
		{
			name: "multiple large lines",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      bytes.Repeat([]byte("test message 01 "), 20000),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(1 * time.Second),
					Data:      bytes.Repeat([]byte("test message 02 "), 20000),
				},
			},
			batchSize:          1,
			expectedWriteCount: 2,
		},
		{
			name: "basic single line with newline",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      []byte("test message line\n"),
				},
			},
			batchSize:          20,
			expectedWriteCount: 1,
		},
		{
			name: "below batch size",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      []byte("test message 1"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(1 * time.Minute),
					Data:      []byte("test message 2"),
				},
			},
			batchSize:          3,
			expectedWriteCount: 2,
		},
		{
			name: "at batch size",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      []byte("test message 1"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(1 * time.Minute),
					Data:      []byte("test message 2"),
				},
			},
			batchSize:          2,
			expectedWriteCount: 2,
		},
		{
			name: "above batch size",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      []byte("test message 1"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(1 * time.Minute),
					Data:      []byte("test message 2"),
				},
			},
			batchSize:          1,
			expectedWriteCount: 2,
		},
		{
			name: "multiple triggered buffer flushes",
			inputs: []protocol.Payload{
				{
					Hostname:  "localhost",
					Timestamp: startTime,
					Data:      []byte("test message 1"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(1 * time.Minute),
					Data:      []byte("test message 2"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(2 * time.Minute),
					Data:      []byte("test message 3"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(3 * time.Minute),
					Data:      []byte("test message 4"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(4 * time.Minute),
					Data:      []byte("test message 5"),
				},
				{
					Hostname:  "localhost",
					Timestamp: startTime.Add(5 * time.Minute),
					Data:      []byte("test message 6"),
				},
			},
			batchSize:          2,
			expectedWriteCount: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Per test mocks
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = logctx.New(ctx, logctx.NSTest, 1, ctx.Done())

			tempDir := t.TempDir()
			outFilePath := filepath.Join(tempDir, "output")

			outFile, err := os.Create(outFilePath)
			if err != nil {
				t.Fatalf("failed to create output file: %v", err)
			}
			err = outFile.Close()
			if err != nil && !errors.Is(err, os.ErrClosed) {
				t.Fatalf("failed to close output file: %v", err)
			}
			defer func() {
				_ = os.Remove(outFilePath)
			}()

			outMod, err := NewOutput(outFilePath, tt.batchSize)
			if err != nil {
				t.Fatalf("failed to create output module: %v", err)
			}

			var foundErrors []string
			var writeCount int
			for _, payload := range tt.inputs {
				written, err := outMod.Write(ctx, payload)
				writeCount += written
				if err != nil {
					foundErrors = append(foundErrors, err.Error())
				}
			}

			cancel()

			written, err := outMod.FlushBuffer() // Normally caller handles this
			if err != nil {
				t.Errorf("failed to flush write buffer to file: %v", err)
			}
			writeCount += written

			err = outMod.Shutdown()
			if err != nil {
				t.Fatalf("failed to shutdown output module: %v", err)
			}

			if writeCount != tt.expectedWriteCount {
				t.Errorf("expected write count to be %d, but got %d",
					tt.expectedWriteCount, writeCount)
			}

			// Validate no errors in ctx logger
			logger := logctx.GetLogger(ctx)
			logLines := logger.GetFormattedLogLines()
			var foundExpectedError bool
			for _, logLine := range logLines {
				if strings.Contains(logLine, "["+logctx.InfoLog+"]") {
					continue
				}
				if tt.expectedErrorMessage != "" && strings.Contains(logLine, tt.expectedErrorMessage) {
					foundExpectedError = true
					continue // Search for other errors
				}
				foundErrors = append(foundErrors, logLine)
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

			// Read file content (validate count and partial match only)
			outFileContents, err := os.ReadFile(outFilePath)
			if err != nil {
				t.Errorf("failed to read output file: %v", err)
			}
			outFileContents = bytes.TrimSuffix(outFileContents, []byte("\n")) // Always delete trailing

			lines := bytes.Split(outFileContents, []byte("\n"))
			if len(lines) != len(tt.inputs) {
				t.Fatalf("Expected %d lines in output file, but found %d lines\nGot:\n%s",
					len(tt.inputs), len(lines), outFileContents)
			}
			for index, line := range lines {
				expectedLineContent := tt.inputs[index].Data
				expectedLineContent = bytes.TrimSuffix(expectedLineContent, []byte("\n")) // Split will have removed newlines prior

				if !bytes.Contains(line, expectedLineContent) {
					t.Errorf("expected output file line %d to contain input index %d content %q, but got %q",
						index, index, string(expectedLineContent), string(line))
				}
			}
		})
	}
}
