package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"slices"
	"syscall"
	"time"
)

// Spawns new "copy" of self with most current executable and waits for signal from child to return
func updateSelf(ctx context.Context) (err error) {
	// Readiness Pipe - Child -> Parent notification (signals when to start parent shutdown)
	readyR, readyW, err := os.Pipe()
	if err != nil {
		err = fmt.Errorf("failed to create readiness pipe for new process: %v\n", err)
		return
	}
	defer readyR.Close()
	defer readyW.Close()

	// Aliveness Pipe - Parent -> Child (signals when to tell systemd that child is new main process)
	// Never close write end, this needs to signal when this process is actually gone. Let OS handle that signal (by cleaning fds)
	parentAliveR, parentAliveW, err := os.Pipe()
	if err != nil {
		err = fmt.Errorf("failed to create readiness pipe for new process: %v\n", err)
		return
	}
	defer parentAliveR.Close()

	// Copy ourselves
	exePath, err := os.Executable()
	if err != nil {
		err = fmt.Errorf("failed to get executable path: %v\n", err)
		parentAliveW.Close()
		return
	}
	args := os.Args
	workingDir, err := os.Getwd()
	if err != nil {
		err = fmt.Errorf("failed to get current working directory: %v\n", err)
		parentAliveW.Close()
		return
	}

	// New executable (the child)
	cmd := exec.Command(exePath, args[1:]...)
	cmd.Dir = workingDir
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// New environment for child
	const fdStartingIndex int = 3
	cmd.ExtraFiles = []*os.File{readyW, parentAliveR}
	readyFDNum := fdStartingIndex + slices.Index(cmd.ExtraFiles, readyW) // Should always be 3 here, but just in case
	parentAliveFDNum := fdStartingIndex + slices.Index(cmd.ExtraFiles, parentAliveR)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=%d", global.EnvNameReadinessFD, readyFDNum),
		fmt.Sprintf("%s=%d", global.EnvNameAlivenessFD, parentAliveFDNum),
	)

	err = cmd.Start()
	if err != nil {
		err = fmt.Errorf("failed to start new process: %v\n", err)
		parentAliveW.Close()
		return
	}
	logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
		"Started replacement child process with PID %d\n", cmd.Process.Pid)

	// Wait for child to successfully start
	err = readinessReceiver(readyR)
	if err != nil {
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
			case <-time.After(5 * time.Second):
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

		parentAliveW.Close()
		return
	}

	// Keep open for the life of this process so child detects parent liveness
	_ = parentAliveW
	return
}
