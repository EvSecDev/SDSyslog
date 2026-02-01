package logctx

import (
	"bytes"
	"context"
	"sdsyslog/internal/global"
	"strings"
	"testing"
)

func TestWatcher_WaitWakeAndDedup(t *testing.T) {
	done := make(chan struct{})

	ctx := New(
		context.Background(),
		global.NSTest,
		5,
		done,
	)

	logger := GetLogger(ctx)
	if logger == nil {
		t.Fatal("logger not found in context")
	}

	var output bytes.Buffer

	// Start watcher
	StartWatcher(logger, &output)

	// Ensure watcher is waiting (queue empty)
	// Nothing logged yet; watcher should be blocked on cond.Wait()

	// Explicit wake should not crash or write anything
	logger.Wake()

	if output.Len() != 0 {
		t.Fatalf("unexpected output before events: %q", output.String())
	}

	// Log repeated messages to trigger dedup
	const repeats = 11
	msg := "duplicate-message"

	for i := 0; i < repeats; i++ {
		LogEvent(ctx, 1, global.InfoLog, msg)
	}

	// Wake watcher in case it is waiting
	logger.Wake()

	// Shut down watcher cleanly
	close(done)
	logger.Wake() // ensure it exits wait

	logger.Wait() // must not block

	out := output.String()
	if out == "" {
		t.Fatal("expected output, got empty string")
	}

	// The original message should appear at least once
	if !strings.Contains(out, msg) {
		t.Fatalf("expected original message in output, got:\n%s", out)
	}

	// Suppression message should appear
	if !strings.Contains(out, "Suppressed") {
		t.Fatalf("expected suppression message, got:\n%s", out)
	}

	// Suppression count should be present
	if !strings.Contains(out, "repeated messages") {
		t.Fatalf("expected repeated message count, got:\n%s", out)
	}
}
