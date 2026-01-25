package lifecycle

import (
	"context"
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
	// Readiness Pipe - Child -> Parent notification (signals when to start parent shutdown)
	readyR, readyW, err := os.Pipe()
	if err != nil {
		err = fmt.Errorf("failed to create readiness pipe for new process: %v", err)
		return
	}
	defer readyR.Close()
	defer readyW.Close()

	// Copy ourselves
	exePath, err := os.Executable()
	if err != nil {
		err = fmt.Errorf("failed to get executable path: %v", err)
		return
	}
	args := os.Args
	workingDir, err := os.Getwd()
	if err != nil {
		err = fmt.Errorf("failed to get current working directory: %v", err)
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

	err = cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start new process: %v", err)
		return
	}
	logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
		"Started temporary child process with PID %d\n", cmd.Process.Pid)

	// Wait for child to successfully start
	err = readinessReceiver(readyR)
	if err != nil {
		terminateChildProcess(ctx, cmd)
		return
	}

	childProc = cmd
	return
}

// Replaces current process with new version from disk.
// Handles restarting daemon if update fails.
// Will only return if update failed.
// Should only be called near normal program exit, successful run will cause program to end execution before return.
func updateAndExit(ctx context.Context, daemonManager DaemonLike, childProc *exec.Cmd) {
	argv := os.Args
	env := os.Environ()

	// Add update environment variable with child process PID
	env = append(env, EnvNameSelfUpdate+"="+strconv.Itoa(childProc.Process.Pid))

	// Will not return. Call below terminates this execution immediately if no error.
	err := syscall.Exec(argv[0], argv, env)
	if err != nil {
		// Cleanup update variable
		err := os.Unsetenv(EnvNameSelfUpdate)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
				"failed to unset environment variable %s (future updates may use wrong PID): %v\n", EnvNameSelfUpdate, err)
		}

		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Self update execve call failed: %v\n", err)
		err = NotifyStatus(ctx, "Reload failed due to internal error. Check daemon logs.")
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify status failed: %v\n", err)
		}

		logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "Restarting daemon\n")

		// Start daemon back up
		// Empty key value avoids re-initializing the decrypt function, since it should already be initialized at this point.
		err = daemonManager.Start(ctx, []byte{})
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Failed to restart daemon after update failure: %v\n", err)
			// Restart failed is fatal at this point, die.
			return
		}

		err = NotifyReady(ctx)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify reload failed: %v\n", err)
		}

		// Remove child
		terminateChildProcess(ctx, childProc)

		// We are already the daemon.Run in this function, so we can skip back to signal processing normally.
	}
}

// Runs post-update (post-execve) actions.
// Kills child PID by env variable.
// All errors non-fatal, sent to context log buffer.
func PostUpdateActions(ctx context.Context) {
	childPID := os.Getenv(EnvNameSelfUpdate)
	if childPID == "" {
		return // not running post update
	}

	// Cleanup update variable
	err := os.Unsetenv(EnvNameSelfUpdate)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"failed to unset environment variable %s (future updates may use wrong PID): %v\n", EnvNameSelfUpdate, err)
	}

	pid, err := strconv.Atoi(childPID)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
			"failed to convert child PID '%s' to integer (child process still running): %v\n", childPID, err)
		return
	}

	// Graceful shutdown of child
	err = syscall.Kill(pid, syscall.SIGTERM)
	if err != nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
			"failed to issue SIGTERM to child PID %d to integer (child process still running): %v\n", pid, err)
		return
	}

	timeout := 10 * time.Second
	deadline := time.Now().Add(timeout)

	var status syscall.WaitStatus

	// Poll for process exit
	for {
		// Try to reap child (non-blocking)
		wpid, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
		if wpid == pid {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
				"Child PID %d exited and was reaped (status=%v)\n", pid, status)
			return
		}

		if err == syscall.ECHILD {
			// Already reaped or not our child anymore
			logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
				"Child PID %d already reaped or no longer a child\n", pid)
			return
		}

		if time.Now().After(deadline) {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
				"Child PID %d did not exit gracefully, forcing shutdown\n", pid)

			// Force kill
			err = syscall.Kill(pid, syscall.SIGKILL)
			if err != nil && err != syscall.ESRCH {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
					"failed to issue SIGKILL to child PID %d (child process might be still running): %v\n", pid, err)
				return
			}

			// After SIGKILL, block until reaped
			for {
				wpid, err := syscall.Wait4(pid, &status, 0, nil)
				if wpid == pid {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
						"Child PID %d killed and reaped (status=%v)\n", pid, status)
					return
				}
				if err != nil && err != syscall.ECHILD {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"failed to wait for child PID %d (child process might be a zombie): %v\n", pid, err)
					return
				}
				if err == syscall.ECHILD {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
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
	killErr := cmd.Process.Signal(syscall.Signal(0))
	if killErr == nil {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
			"Found child PID %d still alive despite not sending readiness signal\n", cmd.Process.Pid)

		// Attempt graceful shutdown
		lerr := cmd.Process.Signal(syscall.SIGTERM)
		if lerr != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
				"Failed to send graceful shutdown signal to child PID %d: %v\n", cmd.Process.Pid, lerr)
		}

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait() // wait for child to exit
		}()

		select {
		case <-time.After(10 * time.Second):
			logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
				"Child PID %d did not exit gracefully, forcing shutdown\n", cmd.Process.Pid)

			lerr := cmd.Process.Kill()
			if lerr != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
					"Failed to force shutdown for child PID %d: %v\n", cmd.Process.Pid, lerr)
			}
			<-done
		case <-done:
			logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
				"Child PID %d exited gracefully\n", cmd.Process.Pid)
		}
	}
}
