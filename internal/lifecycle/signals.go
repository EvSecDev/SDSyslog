package lifecycle

import (
	"context"
	"os"
	"os/exec"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"syscall"
)

type DaemonLike interface {
	Start(context.Context, []byte) (err error)
	Shutdown()
	StartFIPR() (err error)
	StopFIPR()
}

// Process OS signals and handle orchestrating updates via SIGHUP.
// Handles all incoming signals from external sources.
// Initiates daemon shutdown and exits program.
func SignalHandler(ctx context.Context, daemonManager DaemonLike) {
	sigChan := getSigNotifyChannel(DefaultSignalChannelSize,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)

	for {
		// Blocking
		var sig os.Signal
		select {
		case <-ctx.Done():
			return
		case sig = <-sigChan:
		}
		logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
			"Received signal: %v\n", sig)

		recvSignal, ok := sig.(syscall.Signal)
		if !ok {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"Failed to type assert received signal: %v\n", sig)
			continue
		}

		// Reload (Update) signal
		var childProc *exec.Cmd
		if recvSignal == syscall.SIGHUP {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
				"Beginning reload...\n")

			err := NotifyReload(ctx)
			if err != nil {
				logNotifyFailed(ctx, sig, "Systemd notify failed", err)
				continue
			}

			// Start inter-process fragment receiver
			err = daemonManager.StartFIPR()
			if err != nil {
				logNotifyFailed(ctx, sig, "Failed starting FIPR listener", err)
				continue
			}

			childProc, err = preUpdate(ctx)
			if err != nil {
				logNotifyFailed(ctx, sig, "Failed starting temporary process", err)
				continue
			}
		}

		// Initiate daemon shutdown
		daemonManager.Shutdown()

		logger := logctx.GetLogger(ctx)
		logger.Wake() // Logs after here are not guaranteed to print (only if update succeeds)

		// Process update (Replacement)
		if recvSignal == syscall.SIGHUP {
			err := updateAndExit(ctx, daemonManager, childProc.Process.Pid)
			if err != nil {
				// Cleanup update variable
				lerr := os.Unsetenv(EnvNameSelfUpdate)
				if lerr != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog,
						"failed to unset environment variable %s (future updates may use wrong PID): %w\n", EnvNameSelfUpdate, lerr)
				}

				// Start daemon back up
				logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog,
					"Restarting daemon after self update failure\n")
				lerr = daemonManager.Start(ctx, []byte{}) // No private key since it was already initialized
				if lerr != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
						"Failed to restart daemon after update failure: %w (original error: %w)\n", lerr, err)
					// Restart failed is fatal at this point, die.
					return
				}
				terminateChildProcess(ctx, childProc)

				logNotifyFailed(ctx, sig, "Self update (execve) failed with error", err)

				// We are already the daemon.Run in this function, so we can skip back to signal processing normally.
				continue
			}
		}
		return
	}
}
