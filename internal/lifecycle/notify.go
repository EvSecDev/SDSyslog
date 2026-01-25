// Handles operations agnostic of daemon type (Receiver/Sender) to handle program lifecycle (signals, reloads, ect.)
package lifecycle

import (
	"context"
	"fmt"
	"net"
	"os"
	"sdsyslog/internal/global"
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
		err = fmt.Errorf("notify dial failed: %v", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg))
	if err != nil {
		err = fmt.Errorf("notify write failed: %v", err)
		return
	}

	logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog, "Successfully notified systemd with message '%s'\n", msg)
	return
}
