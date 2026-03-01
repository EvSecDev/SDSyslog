package lifecycle

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sdsyslog/internal/logctx"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPostUpdateActions_ErrorPaths(t *testing.T) {

	type testCase struct {
		name string

		envValue string

		startFIPRErr error
		killErr      error
		waitErr      error
		waitPID      int

		expectedErr string
	}

	tests := []testCase{
		{
			name:        "invalid PID string",
			envValue:    "notanumber",
			expectedErr: "failed to convert child PID",
		},
		{
			name:        "SIGTERM failure",
			envValue:    "123",
			killErr:     errors.New("sigterm failed"),
			expectedErr: "failed to issue SIGTERM to child PID",
		},
		{
			name:        "wait4 error",
			envValue:    "123",
			waitErr:     errors.New("wait failed"),
			expectedErr: "f",
		},
		{
			name:        "already reaped (ECHILD)",
			envValue:    "123",
			waitErr:     syscall.ECHILD,
			expectedErr: "already reaped or no longer a child",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			baseCtx := context.Background()
			ctx := logctx.New(baseCtx, "test", logctx.VerbosityStandard, nil)

			err := os.Setenv(EnvNameSelfUpdate, tt.envValue)
			if err != nil {
				t.Fatalf("unexpected error setting env var: %v", err)
			}
			defer func() {
				err := os.Unsetenv(EnvNameSelfUpdate)
				if err != nil {
					t.Fatalf("failed unsetting env var: %v", err)
				}
			}()

			origKill := syscallKill
			syscallKill = func(int, syscall.Signal) error {
				return tt.killErr
			}
			defer func() { syscallKill = origKill }()

			origWait := syscallWait4
			syscallWait4 = func(pid int, w *syscall.WaitStatus, options int, r *syscall.Rusage) (int, error) {
				return tt.waitPID, tt.waitErr
			}
			defer func() { syscallWait4 = origWait }()

			mock := daemonFuncAdapter{
				startFIPRFunc: func() error { return tt.startFIPRErr },
			}

			PostUpdateActions(ctx, mock, 10*time.Millisecond)

			logger := logctx.GetLogger(ctx)
			lines := logger.GetFormattedLogLines()
			var foundMatch bool
			for _, line := range lines {
				if !strings.Contains(line, tt.expectedErr) {
					continue
				}
				foundMatch = true
			}
			if tt.expectedErr != "" && !foundMatch {
				t.Errorf("expected error %q, but found none", tt.expectedErr)
			}
		})
	}
}

func TestTerminateChildProcess_ErrorPaths(t *testing.T) {
	type testCase struct {
		name string

		signal0Err error
		sigtermErr error
		killErr    error
		waitDelay  time.Duration
	}

	tests := []testCase{
		{
			name:       "child not alive",
			signal0Err: errors.New("not running"),
		},
		{
			name:       "sigterm failure",
			sigtermErr: errors.New("sigterm failed"),
		},
		{
			name:      "force kill failure",
			killErr:   errors.New("kill failed"),
			waitDelay: 11 * time.Second,
		},
		{
			name: "graceful exit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			baseCtx := context.Background()
			ctx := logctx.New(baseCtx, "test", logctx.VerbosityStandard, nil)

			cmd := &exec.Cmd{
				Process: &os.Process{Pid: 999},
			}

			origSignal := cmdProcSignal
			cmdProcSignal = func(*exec.Cmd, os.Signal) error {
				return tt.signal0Err
			}
			defer func() { cmdProcSignal = origSignal }()

			origKill := cmdProcKill
			cmdProcKill = func(*exec.Cmd) error {
				return tt.killErr
			}
			defer func() { cmdProcKill = origKill }()

			terminateChildProcess(ctx, cmd)
		})
	}
}
