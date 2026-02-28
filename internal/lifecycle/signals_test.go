package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestSignalHandling(t *testing.T) {
	tests := []struct {
		name                  string
		signal                os.Signal
		fail                  failureConfig
		expectExit            bool
		expectDaemonShutdown  bool
		expectDaemonRestart   bool
		expectExecCall        bool
		expectedNotifies      int
		expectNotifyStatusMsg string
		expectNotifyREADY     bool
		expectNotifyRELOAD    bool
	}{
		{
			name:                 "Success: regular shutdown",
			signal:               syscall.SIGTERM,
			expectExit:           true,
			expectDaemonShutdown: true,
		},
		{
			name:                 "Success: update (exec works)",
			signal:               syscall.SIGHUP,
			expectExit:           true,
			expectExecCall:       true,
			expectedNotifies:     1,
			expectNotifyRELOAD:   true,
			expectDaemonShutdown: true,
		},
		{
			name:   "Failure: update (exec fails)",
			signal: syscall.SIGHUP,
			fail: failureConfig{
				execErr: os.ErrPermission,
			},
			expectedNotifies:      3,
			expectNotifyRELOAD:    true,
			expectNotifyStatusMsg: "Reload failed due to internal error. Check daemon logs.",
			expectNotifyREADY:     true,
			expectExecCall:        true,
			expectDaemonShutdown:  true,
			expectDaemonRestart:   true,
		},
		{
			name:   "Failure: fipr startup",
			signal: syscall.SIGHUP,
			fail: failureConfig{
				startFIPRErr: os.ErrPermission,
			},
			expectedNotifies:      3,
			expectNotifyRELOAD:    true,
			expectNotifyStatusMsg: "Reload failed due to internal error. Check daemon logs.",
			expectNotifyREADY:     true,
		},
		{
			name:   "Failure: child process start",
			signal: syscall.SIGHUP,
			fail: failureConfig{
				cmdStartErr: os.ErrNotExist,
			},
			expectedNotifies:      3,
			expectNotifyRELOAD:    true,
			expectNotifyStatusMsg: "Reload failed due to internal error. Check daemon logs.",
			expectNotifyREADY:     true,
		},
		{
			name:   "Failure: daemon restart",
			signal: syscall.SIGHUP,
			fail: failureConfig{
				execErr:    os.ErrInvalid,
				restartErr: fmt.Errorf("invalid parameter"),
			},
			expectExit:           true,
			expectedNotifies:     1,
			expectNotifyRELOAD:   true,
			expectExecCall:       true,
			expectDaemonShutdown: true,
			expectDaemonRestart:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := logctx.New(baseCtx, "test", global.VerbosityStandard, baseCtx.Done())
			ctx = context.WithValue(ctx, global.CtxModeKey, global.RecvMode)

			// Setup real notify socket
			socketPath, msgChan, cleanup := setupNotifySocket(t)
			defer cleanup()
			os.Setenv("NOTIFY_SOCKET", socketPath)
			defer os.Unsetenv("NOTIFY_SOCKET")

			// Mock low-level dependencies
			mockReader, mockWriter, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			origOSPipe := osPipe
			osPipe = func() (reader *os.File, writer *os.File, err error) {
				reader = mockReader
				writer = mockWriter
				return
			}
			defer mockReader.Close()
			defer mockWriter.Close()
			defer func() { osPipe = origOSPipe }()

			origExe := osExecutable
			osExecutable = func() (string, error) {
				return "/fake/exe", nil
			}
			defer func() { osExecutable = origExe }()

			origCmdStart := cmdStart
			cmdStart = func(cmd *exec.Cmd) error {
				cmd.Process = &os.Process{Pid: 12345}
				return tt.fail.cmdStartErr
			}
			defer func() { cmdStart = origCmdStart }()

			origExec := syscallExec
			var execCalled bool
			syscallExec = func(string, []string, []string) error {
				execCalled = true
				return tt.fail.execErr
			}
			defer func() { syscallExec = origExec }()

			fakeChan := make(chan os.Signal)
			origSig := getSigNotifyChannel
			getSigNotifyChannel = func(int, ...os.Signal) chan os.Signal {
				return fakeChan
			}
			defer func() { getSigNotifyChannel = origSig }()

			// Spawn goroutine to simulate child signaling readiness
			if tt.fail.startFIPRErr == nil && tt.fail.cmdStartErr == nil {
				go func() {
					time.Sleep(50 * time.Millisecond)
					mockWriter.Write([]byte(ReadyMessage))
				}()
			}

			var shutdownCalled, restartCalled bool
			mock := daemonFuncAdapter{
				shutdownFunc: func() {
					shutdownCalled = true
				},
				startFunc: func(context.Context, []byte) error {
					restartCalled = true
					return tt.fail.restartErr
				},
				startFIPRFunc: func() error {
					return tt.fail.startFIPRErr
				},
			}

			// Run handler
			sigCtx, sigCancel := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				SignalHandler(sigCtx, mock)
				close(done)
			}()

			// Send update signal
			fakeChan <- tt.signal

			// Wait for handler exit
			if tt.expectExit {
				select {
				case <-done:
				case <-time.After(500 * time.Millisecond):
					sigCancel()
					t.Fatalf("signal handler did not exit after test expected graceful exit")
				}
				sigCancel()
			} else {
				// Kill through ctx
				sigCancel()
				select {
				case <-done:
				case <-time.After(100 * time.Millisecond):
					t.Fatalf("signal handler did not exit normally after context cancellation")
				}
			}

			// Check logger for errors
			logger := logctx.GetLogger(ctx)
			logLines := logger.GetFormattedLogLines()
			var foundErrors []string
			for _, line := range logLines {
				if strings.Contains(line, "["+global.InfoLog+"]") {
					continue
				}
				if checkLogForErrors(line, tt.fail) {
					continue
				}
				foundErrors = append(foundErrors, line)
			}
			if len(foundErrors) > 0 {
				t.Error(" Found errors in logger")
				for _, log := range foundErrors {
					t.Errorf("  %s", log)
				}
			}

			if shutdownCalled != tt.expectDaemonShutdown {
				t.Fatalf("daemon shutdown call mismatch: expected call=%v but got %v", tt.expectDaemonShutdown, shutdownCalled)
			}
			if restartCalled != tt.expectDaemonRestart {
				t.Fatalf("daemon restart call mismatch: expected call=%v but got %v", tt.expectDaemonRestart, restartCalled)
			}
			if execCalled != tt.expectExecCall {
				t.Fatalf("execve call mismatch: expected call=%v but got %v", tt.expectExecCall, execCalled)
			}

			// Collect notify messages
			received := make([]string, 0, tt.expectedNotifies)
			for len(received) < tt.expectedNotifies {
				select {
				case msg := <-msgChan:
					received = append(received, msg)
				case <-time.After(time.Second):
					t.Fatalf("timed out waiting for notify messages: expected %d notifications, but only got %d", tt.expectedNotifies, len(received))
				}
			}

			// Validate notify messages
			var foundReload, foundReady bool
			var foundStatusMsg string
			for _, msg := range received {
				if strings.Contains(msg, "RELOADING=1") {
					foundReload = true
				}
				if strings.Contains(msg, "STATUS=") {
					fields := strings.Split(msg, "=")
					foundStatusMsg = fields[1]
				}
				if strings.Contains(msg, "READY=1") {
					foundReady = true
				}
			}

			if foundReload != tt.expectNotifyRELOAD {
				t.Fatalf("Notify RELOADING mismatch got=%v expected=%v", foundReload, tt.expectNotifyRELOAD)
			}
			if foundStatusMsg != tt.expectNotifyStatusMsg {
				t.Fatalf("Notify STATUS mismatch got=%q expected=%q", foundStatusMsg, tt.expectNotifyStatusMsg)
			}
			if foundReady != tt.expectNotifyREADY {
				t.Fatalf("Notify READY mismatch got=%v expected=%v", foundReady, tt.expectNotifyREADY)
			}
		})
	}
}
