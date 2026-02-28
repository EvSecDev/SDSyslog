// Handles operations agnostic of daemon type (Receiver/Sender) to handle program lifecycle (signals, reloads, ect.)
package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os"
	"sdsyslog/internal/logctx"

	"golang.org/x/sys/unix"
)

// Sends RELOADING=1 to systemd to indicate service reload in progress.
func NotifyReload(ctx context.Context) (err error) {
	var ts unix.Timespec
	err = unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts)
	if err != nil {
		return
	}

	usec := ts.Sec*1_000_000 + int64(ts.Nsec)/1_000

	err = notify(ctx, fmt.Sprintf("RELOADING=1\nMONOTONIC_USEC=%d", usec))
	return
}

// Sends READY=1 to systemd to indicate service startup complete.
func NotifyReady(ctx context.Context) (err error) {
	// Temporary child processes for updates never send ready
	fdStr := os.Getenv(EnvNameReadinessFD)
	if fdStr != "" {
		return // running under updater
	}

	err = notify(ctx, "READY=1")
	return
}

// Sends custom status message to systemd for context.
func NotifyStatus(ctx context.Context, msg string) (err error) {
	err = notify(ctx, "STATUS="+msg)
	return
}

// Logs failure and handles notifying systemd of failure.
func logNotifyFailed(ctx context.Context, sig os.Signal, msg string, err error) {
	logctx.LogStdErr(ctx, "%s: %w\n", msg, err)

	err = NotifyStatus(ctx, "Reload failed due to internal error. Check daemon logs.")
	if err != nil {
		logctx.LogStdWarn(ctx, "Systemd notify status failed: %v\n", sig)
	}
	err = NotifyReady(ctx)
	if err != nil {
		logctx.LogStdWarn(ctx, "Systemd notify reload failed: %v\n", sig)
	}
}

// Sends a raw sd_notify message.
// If NOTIFY_SOCKET is unset, this is a no-op and returns nil.
func notify(ctx context.Context, msg string) (err error) {
	sockPath := os.Getenv("NOTIFY_SOCKET")
	if sockPath == "" {
		// Not running under systemd
		return
	}

	addr := &net.UnixAddr{
		Name: sockPath,
		Net:  "unixgram",
	}

	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		err = fmt.Errorf("notify dial failed: %w", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg))
	if err != nil {
		err = fmt.Errorf("notify write failed: %w", err)
		return
	}

	logctx.LogEvent(ctx, logctx.VerbosityProgress, logctx.InfoLog, "Successfully notified systemd with message '%s'\n", msg)
	return
}
