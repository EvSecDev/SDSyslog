package inotify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/tests/utils"
	"testing"
	"time"
)

func TestInotify(t *testing.T) {
	tests := []struct {
		name                  string
		changeFunc            func(filePath string) (err error)
		expectedErrorMessage  string
		expectedChangeSignals int
		expectedRotateSignals int
	}{
		{
			name: "File write",
			changeFunc: func(filePath string) (err error) {
				err = os.WriteFile(filePath, []byte("test message"), 0644)
				return
			},
			expectedChangeSignals: 2,
		},
		{
			name: "File read",
			changeFunc: func(filePath string) (err error) {
				_, err = os.ReadFile(filePath)
				return
			},
			expectedChangeSignals: 0,
		},
		{
			name: "File open and close",
			changeFunc: func(filePath string) (err error) {
				file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
				if err != nil {
					return
				}
				err = file.Close()
				return
			},
			expectedChangeSignals: 0,
		},
		{
			name: "File manual write",
			changeFunc: func(filePath string) (err error) {
				file, err := os.OpenFile(filePath, os.O_RDWR, 0)
				if err != nil {
					return
				}
				defer func() {
					lerr := file.Close()
					if err == nil && lerr != nil {
						err = lerr
					}
				}()
				_, err = file.Write([]byte("test message"))
				if err != nil {
					return
				}
				return
			},
			expectedChangeSignals: 1,
		},
		{
			name: "File moved then recreated",
			changeFunc: func(filePath string) (err error) {
				err = os.Rename(filePath, filePath+".1")
				if err != nil {
					return
				}
				file, err := os.Create(filePath)
				if err != nil {
					return
				}
				err = file.Close()
				return
			},
			expectedChangeSignals: 0,
			expectedRotateSignals: 1,
		},
		{
			name: "File copied then truncated",
			changeFunc: func(filePath string) (err error) {
				file, err := os.OpenFile(filePath, os.O_RDWR, 0)
				if err != nil {
					return
				}
				defer func() {
					lerr := file.Close()
					if err == nil && lerr != nil && !errors.Is(lerr, os.ErrClosed) {
						err = lerr
					}
				}()
				newFile, err := os.Create(filePath + ".1")
				if err != nil {
					return
				}
				_, err = io.Copy(newFile, file)
				if err != nil {
					return
				}
				err = file.Close()
				if err != nil {
					return
				}
				err = os.Truncate(filePath, 0)
				if err != nil {
					return
				}
				return
			},
			expectedChangeSignals: 1, // Contents at existing inode were removed
			expectedRotateSignals: 0, // Inode has not changed
		},
		{
			name: "File deleted",
			changeFunc: func(filePath string) (err error) {
				err = os.Remove(filePath)
				return
			},
			expectedChangeSignals: 0,
			expectedRotateSignals: 0, // deletion alone does not create new inode
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

			logFile, err := os.Create(logFilePath)
			if err != nil {
				t.Fatalf("failed to create log file: %v", err)
			}
			err = logFile.Close()
			if err != nil {
				t.Fatalf("failed to close log file: %v", err)
			}

			watcher, err := New(ctx, logFilePath)
			if err != nil {
				t.Fatalf("failed to create new watcher: %v", err)
			}

			watcher.Start()

			// Run test changes
			errChan := make(chan error, 1)
			go func() {
				err := tt.changeFunc(logFilePath)
				if err != nil {
					errChan <- fmt.Errorf("failed to run change function: %v", err)
				}
				watcher.Stop()
			}()

			var gotChangeSignals, gotRotateSignals int
			timeout := time.After(200 * time.Millisecond)

			for gotChangeSignals < tt.expectedChangeSignals || gotRotateSignals < tt.expectedRotateSignals {
				var done bool
				select {
				case <-watcher.fileHasChanged:
					gotChangeSignals++
				case <-watcher.fileHasRotated:
					gotRotateSignals++
				case <-timeout:
					done = true
				}
				if done {
					break
				}
			}

			// Test func errors
			if len(errChan) > 0 {
				err := <-errChan
				if err != nil {
					t.Fatalf("%v", err)
				}
			}

			if gotChangeSignals != tt.expectedChangeSignals {
				t.Errorf("expected %d change signals, got %d signals", tt.expectedChangeSignals, gotChangeSignals)
			}
			if gotRotateSignals != tt.expectedRotateSignals {
				t.Errorf("expected %d rotate signals, got %d signals", tt.expectedRotateSignals, gotRotateSignals)
			}

			// Validate no errors in log
			_, err = utils.MatchLogCtxErrors(ctx, tt.expectedErrorMessage, nil)
			if err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}
