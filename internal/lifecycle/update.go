package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"slices"
	"strconv"
	"syscall"
	"time"
)

// Creates new daemon process as child to temporarily handle traffic during main process update.
// Only returns after child process is fully started and taking traffic (or error).
func preUpdate(ctx context.Context) (childProc *exec.Cmd, err error) {
	// Sender daemons will duplicate messages if we attempt a 2 stage update.
	// They can proceed directly to shutdown-replacement since inputs are stateful.
	mode, ok := ctx.Value(global.CtxModeKey).(string)
	if !ok {
		err = fmt.Errorf("attempted retrieval and assertion of context mode key failed")
		return
	}
	if mode == global.SendMode {
		// No-op for sender
		return
	}

	// Readiness Pipe - Child -> Parent notification (signals when to start parent shutdown)
	readyR, readyW, err := osPipe()
	if err != nil {
		err = fmt.Errorf("failed to create readiness pipe for new process: %w", err)
		return
	}
	defer func() {
		_ = readyR.Close()
		_ = readyW.Close()
	}()

	// Copy ourselves
	exePath, err := osExecutable()
	if err != nil {
		err = fmt.Errorf("failed to get executable path: %w", err)
		return
	}
	args := os.Args
	workingDir, err := os.Getwd()
	if err != nil {
		err = fmt.Errorf("failed to get current working directory: %w", err)
		return
	}

	// Temporary executable
	cmd := exec.Command(exePath, args[1:]...)
	cmd.Dir = workingDir
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// New environment for temp process
	const fdStartingIndex int = 3
	cmd.ExtraFiles = []*os.File{readyW}
	readyFDNum := fdStartingIndex + slices.Index(cmd.ExtraFiles, readyW) // Should always be 3 here, but just in case
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=%d", EnvNameReadinessFD, readyFDNum),
	)

	err = cmdStart(cmd)
	if err != nil {
		err = fmt.Errorf("failed to start new process: %w", err)
		return
	}
	logctx.LogStdInfo(ctx,
		"Started temporary child process with PID %d\n", cmd.Process.Pid)

	// Wait for child to successfully start
	err = readinessReceiver(readyR)
	if err != nil {
		terminateChildProcess(ctx, cmd)
		return
	}

	logctx.LogStdInfo(ctx, "Temporary Child Process is ready, proceeding with update\n")
	childProc = cmd
	return
}

// Replaces current process with new version from disk.
// Will only return if update failed.
// Should only be called near normal program exit, successful run will cause program to end execution before return.
func updateAndExit(childPID int) (err error) {
	argv := os.Args
	env := os.Environ()

	if childPID != 0 {
		// Add update environment variable with child process PID
		env = append(env, EnvNameSelfUpdate+"="+strconv.Itoa(childPID))
	}

	// Will not return. Call below terminates this execution immediately if no error.
	err = syscallExec(argv[0], argv, env)
	if err != nil {
		return
	}
	// Should never get here
	return
}

// Runs pre-full-startup actions that a temporary child process running under an update should do.
// No-op when the temp child env variable is not present.
func TempChildActions(ctx context.Context, daemonManager DaemonLike) {
	_, isTempChild := os.LookupEnv(EnvNameReadinessFD)
	if !isTempChild {
		return // not running as temp process during update
	}

	// Start the receiver to get fragments from the shutting down main process
	err := daemonManager.StartFIPR()
	if err != nil {
		logctx.LogStdWarn(ctx,
			"failed to start inter-process fragment temporary receiver (multi-packet messages might be missing fragments): %w\n", err)
	}

	// FIPR shutdown can be handled normally through daemon shutdown.
}

// Runs post-update (post-execve) actions.
// Kills child PID by env variable.
// All errors non-fatal, sent to context log buffer.
func PostUpdateActions(ctx context.Context, daemonManager DaemonLike, timeout time.Duration) {
	childPID := os.Getenv(EnvNameSelfUpdate)
	if childPID == "" {
		return // not running post update
	}

	// Need to restart FIPR receiver for receiving fragments from child process
	err := daemonManager.StartFIPR()
	if err != nil {
		logctx.LogStdWarn(ctx,
			"failed to start inter-process fragment receiver (multi-packet messages might be missing fragments): %w\n", err)
	}
	// Shutdown FIPR when done - not needed when another process is not running
	defer daemonManager.StopFIPR()

	// Cleanup update variable
	err = os.Unsetenv(EnvNameSelfUpdate)
	if err != nil {
		logctx.LogStdWarn(ctx,
			"failed to unset environment variable %s (future updates may use wrong PID): %w\n", EnvNameSelfUpdate, err)
	}

	pid, err := strconv.Atoi(childPID)
	if err != nil {
		logctx.LogStdErr(ctx,
			"failed to convert child PID '%s' to integer (child process still running): %w\n", childPID, err)
		return
	}

	// Graceful shutdown of child
	err = syscallKill(pid, syscall.SIGTERM)
	if err != nil {
		logctx.LogStdErr(ctx,
			"failed to issue SIGTERM to child PID %d to integer (child process still running): %w\n", pid, err)
		return
	}

	deadline := time.Now().Add(timeout)

	var status syscall.WaitStatus

	// Poll for process exit
	for {
		// Try to reap child (non-blocking)
		wpid, err := syscallWait4(pid, &status, syscall.WNOHANG, nil)
		if wpid == pid {
			logctx.LogStdInfo(ctx,
				"Child PID %d exited and was reaped (status=%v)\n", pid, status)
			return
		}

		if errors.Is(err, syscall.ECHILD) {
			// Already reaped or not our child anymore
			logctx.LogStdInfo(ctx,
				"Child PID %d already reaped or no longer a child\n", pid)
			return
		}

		if time.Now().After(deadline) {
			logctx.LogStdWarn(ctx,
				"Child PID %d did not exit gracefully, forcing shutdown\n", pid)

			// Force kill
			err = syscallKill(pid, syscall.SIGKILL)
			if err != nil && err != syscall.ESRCH {
				logctx.LogStdErr(ctx,
					"failed to issue SIGKILL to child PID %d (child process might be still running): %w\n", pid, err)
				return
			}

			// After SIGKILL, block until reaped
			for {
				wpid, err := syscallWait4(pid, &status, 0, nil)
				if wpid == pid {
					logctx.LogStdWarn(ctx,
						"Child PID %d killed and reaped (status=%v)\n", pid, status)
					return
				}
				if err != nil && err != syscall.ECHILD {
					logctx.LogStdErr(ctx,
						"failed to wait for child PID %d (child process might be a zombie): %w\n", pid, err)
					return
				}
				if err == syscall.ECHILD {
					logctx.LogStdInfo(ctx,
						"Child PID %d force killed and was cleaned up by something else\n", pid)
					break
				}
			}

			break
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Attempts graceful shutdown of child process, force kills if timeout.
// All events (including errors) logged to context log buffer.
func terminateChildProcess(ctx context.Context, cmd *exec.Cmd) {
	if cmd == nil {
		// Successful (already exited)
		return
	}

	killErr := cmdProcSignal(cmd, syscall.Signal(0))
	if killErr == nil {
		if cmd == nil {
			// Successful (exited between check and now)
			return
		}

		logctx.LogStdWarn(ctx,
			"Found child PID %d still alive despite not sending readiness signal\n", cmd.Process.Pid)

		// Attempt graceful shutdown
		lerr := cmdProcSignal(cmd, syscall.SIGTERM)
		if lerr != nil {
			logctx.LogStdWarn(ctx,
				"Failed to send graceful shutdown signal to child PID %d: %w\n", cmd.Process.Pid, lerr)
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait() // wait for child to exit
		}()

		select {
		case <-time.After(10 * time.Second):
			logctx.LogStdWarn(ctx,
				"Child PID %d did not exit gracefully, forcing shutdown\n", cmd.Process.Pid)

			lerr := cmdProcKill(cmd)
			if lerr != nil {
				logctx.LogStdWarn(ctx,
					"Failed to force shutdown for child PID %d: %w\n", cmd.Process.Pid, lerr)
			}
			<-done
		case <-done:
			logctx.LogStdInfo(ctx,
				"Child PID %d exited gracefully\n", cmd.Process.Pid)
		}
	}
}
