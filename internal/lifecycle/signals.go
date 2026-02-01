package lifecycle

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"syscall"
)

type DaemonLike interface {
	Start(context.Context, []byte) (err error)
	Shutdown()
}

// Handles all incoming signals from external sources.
// Initiates daemon shutdown and exits program.
func SignalHandler(ctx context.Context, daemonManager DaemonLike) {
	// Channel for handling interrupt signals
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		// Blocking
		sig := <-sigChan
		logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "Received signal: %v\n", sig)

		recvSignal, ok := sig.(syscall.Signal)
		if !ok {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Failed to type assert received signal: %v\n", sig)
			continue
		}

		// Reload (Update) signal
		var childProc *exec.Cmd
		if recvSignal == syscall.SIGHUP {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "Beginning reload...\n")
			err := NotifyReload(ctx)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Systemd notify failed: %v\n", sig)

				err = NotifyStatus(ctx, "Reload failed due to internal error. Check daemon logs.")
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify status failed: %v\n", sig)
				}
				err = NotifyReady(ctx)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify reload failed: %v\n", sig)
				}
				continue
			}

			childProc, err = preUpdate(ctx)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Reload Error: %w\n", err)

				err = NotifyStatus(ctx, "Reload failed due to internal error. Check daemon logs.")
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify status failed: %v\n", sig)
				}
				err = NotifyReady(ctx)
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify reload failed: %v\n", sig)
				}
				continue
			}

			// Cleared to replace this process
		}

		// Initiate daemon shutdown
		daemonManager.Shutdown()

		logger := logctx.GetLogger(ctx)
		logger.Wake() // Logs after here are not guaranteed to print (if update succeeds)

		// Process update (Replacement)
		if recvSignal == syscall.SIGHUP {
			updateAndExit(ctx, daemonManager, childProc)
			// If execution got here, there was an update error. Handled inside ^, so just go back to signal processing.
			continue
		}
		return
	}
}
