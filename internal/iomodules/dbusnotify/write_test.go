package dbusnotify

import (
	"context"
	"errors"
	"os"
	"sdsyslog/internal/iomodules"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = logctx.New(ctx, logctx.NSTest, 1, ctx.Done())

	mod, err := NewOutput(true)
	if err != nil {
		if errors.Is(err, unix.EPIPE) || errors.Is(err, unix.ECONNRESET) {
			// Test was run on a system with dbus, but not a fully "functional" one
			t.Logf("received broken pipe/connection reset when attempting to dial session DBUS, not running test.")
			return
		} else if strings.HasSuffix(err.Error(), "refusing to start notification output") {
			// Test was run on a system without dbus
			t.Logf("%v", err)
			return
		}
		t.Fatalf("failed to create output module: %v", err)
	}

	selfPid := strconv.Itoa(os.Getpid())

	testMsg := protocol.Payload{
		Timestamp: time.Now(),
		Hostname:  "localhost",
		CustomFields: map[string]any{
			iomodules.CFappname:   "TestWrite",
			iomodules.CFseverity:  "info",
			iomodules.CFfacility:  "user",
			iomodules.CFprocessid: selfPid,
		},
		Data: []byte("hello from dbusnotify unit test"),
	}

	_, err = mod.Write(ctx, &testMsg)
	if err != nil {
		t.Fatalf("failed to write to notifier: %v", err)
	}

	err = mod.Shutdown()
	if err != nil {
		t.Fatalf("failed to shutdown notifier module: %v", err)
	}
}
