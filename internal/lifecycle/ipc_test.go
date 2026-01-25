package lifecycle

import (
	"io"
	"os"
	"sdsyslog/internal/global"
	"strconv"
	"testing"
	"time"
)

func TestReadinessHandshake(t *testing.T) {
	type testCase struct {
		name string
		run  func(t *testing.T)
	}

	tests := []testCase{
		{
			name: "receiver success",
			run: func(t *testing.T) {
				r, w, _ := os.Pipe()
				defer r.Close()
				defer w.Close()

				go func() {
					time.Sleep(10 * time.Millisecond)
					w.Write([]byte(global.ReadyMessage))
				}()

				if err := readinessReceiver(r); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "receiver wrong message",
			run: func(t *testing.T) {
				r, w, _ := os.Pipe()
				defer r.Close()
				defer w.Close()

				go func() {
					w.Write([]byte("WRONGMSG"))
				}()

				if err := readinessReceiver(r); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "receiver short read",
			run: func(t *testing.T) {
				r, w, _ := os.Pipe()
				defer r.Close()

				go func() {
					w.Write([]byte(global.ReadyMessage[:1]))
					w.Close()
				}()

				if err := readinessReceiver(r); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "sender no env",
			run: func(t *testing.T) {
				os.Unsetenv(global.EnvNameReadinessFD)
				if err := ReadinessSender(); err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
			},
		},
		{
			name: "sender invalid env",
			run: func(t *testing.T) {
				os.Setenv(global.EnvNameReadinessFD, "bad")
				defer os.Unsetenv(global.EnvNameReadinessFD)

				if err := ReadinessSender(); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "sender bad fd",
			run: func(t *testing.T) {
				os.Setenv(global.EnvNameReadinessFD, "999999")
				defer os.Unsetenv(global.EnvNameReadinessFD)

				if err := ReadinessSender(); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "sender success",
			run: func(t *testing.T) {
				r, w, _ := os.Pipe()
				defer r.Close()

				os.Setenv(global.EnvNameReadinessFD, strconv.Itoa(int(w.Fd())))
				defer os.Unsetenv(global.EnvNameReadinessFD)

				if err := ReadinessSender(); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				buf := make([]byte, len(global.ReadyMessage))
				if _, err := io.ReadFull(r, buf); err != nil {
					t.Fatalf("read failed: %v", err)
				}

				if string(buf) != global.ReadyMessage {
					t.Fatalf("expected %q, got %q", global.ReadyMessage, buf)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
