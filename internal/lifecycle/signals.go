package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"syscall"
)

type DaemonLike interface {
	Shutdown()
}

// Handles all incoming signals from external sources.
// Initiates daemon shutdown and exits program.
func SignalHandler(ctx context.Context, daemonManager DaemonLike) {
	// Channel for handling interrupt signals
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

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
		if recvSignal == syscall.SIGHUP {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "Beginning reload...\n")
			err := NotifyReload(ctx)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Systemd notify failed: %v\n", sig)

				err = NotifyStatus(ctx, "Reload failed due to internal error. Check daemon logs.")
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify status failed: %v\n", sig)
				}
				err = NotifyReady(ctx) // Have to send ready to avoid getting old process killed
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify reload failed: %v\n", sig)
				}
				continue
			}

			err = updateSelf(ctx)
			if err != nil {
				logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Reload Error: %v\n", err)

				err = NotifyStatus(ctx, "Reload failed due to internal error. Check daemon logs.")
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify status failed: %v\n", sig)
				}
				err = NotifyReady(ctx) // Have to send ready to avoid getting old process killed
				if err != nil {
					logctx.LogEvent(ctx, global.VerbosityStandard, global.WarnLog, "Systemd notify reload failed: %v\n", sig)
				}
				continue
			}

			// Cleared to shutdown this process
		}

		// Initiate daemon shutdown
		daemonManager.Shutdown()
		logger := logctx.GetLogger(ctx)
		logger.Wake()
		return
	}
}
