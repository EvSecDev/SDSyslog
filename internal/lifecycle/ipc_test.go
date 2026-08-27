package lifecycle

import (
	"io"
	"os"
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
				t.Helper()
				reader, writer, _ := os.Pipe()
				defer func() {
					err := reader.Close()
					if err != nil {
						t.Fatalf("failed closing pipe reader: %v", err)
					}
					err = writer.Close()
					if err != nil {
						t.Fatalf("failed closing pipe writer: %v", err)
					}
				}()

				go func() {
					time.Sleep(10 * time.Millisecond)
					_, err := writer.Write([]byte(ReadyMessage))
					if err != nil {
						t.Logf("unexpected error writing to pipe: %v", err)
					}
				}()

				if err := readinessReceiver(reader); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			},
		},
		{
			name: "receiver wrong message",
			run: func(t *testing.T) {
				t.Helper()
				reader, writer, _ := os.Pipe()
				defer func() {
					err := reader.Close()
					if err != nil {
						t.Fatalf("failed closing pipe reader: %v", err)
					}
					err = writer.Close()
					if err != nil {
						t.Fatalf("failed closing pipe writer: %v", err)
					}
				}()

				go func() {
					_, err := writer.Write([]byte("WRONGMSG"))
					if err != nil {
						t.Logf("unexpected error writing to pipe: %v", err)
					}
				}()

				if err := readinessReceiver(reader); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "receiver short read",
			run: func(t *testing.T) {
				t.Helper()
				reader, writer, _ := os.Pipe()
				defer func() {
					err := reader.Close()
					if err != nil {
						t.Fatalf("failed closing pipe reader: %v", err)
					}
				}()

				go func() {
					_, err := writer.Write([]byte(ReadyMessage[:1]))
					if err != nil {
						t.Logf("unexpected error writing to pipe: %v", err)
					}
					err = writer.Close()
					if err != nil {
						t.Logf("failed closing pipe writer: %v", err)
					}
				}()

				if err := readinessReceiver(reader); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "sender no env",
			run: func(t *testing.T) {
				t.Helper()
				err := os.Unsetenv(EnvNameReadinessFD)
				if err != nil {
					t.Fatalf("unexpected error setting environment variable: %v", err)
				}
				if err := ReadinessSender(); err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
			},
		},
		{
			name: "sender invalid env",
			run: func(t *testing.T) {
				t.Helper()
				err := os.Setenv(EnvNameReadinessFD, "bad")
				if err != nil {
					t.Fatalf("unexpected error setting environment variable: %v", err)
				}
				defer func() {
					err := os.Unsetenv(EnvNameReadinessFD)
					if err != nil {
						t.Fatalf("failed unsetting env variable: %v", err)
					}
				}()

				if err := ReadinessSender(); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "sender bad fd",
			run: func(t *testing.T) {
				t.Helper()
				err := os.Setenv(EnvNameReadinessFD, "999999")
				if err != nil {
					t.Fatalf("unexpected error setting environment variable: %v", err)
				}
				defer func() {
					err := os.Unsetenv(EnvNameReadinessFD)
					if err != nil {
						t.Fatalf("failed unsetting env variable: %v", err)
					}
				}()

				if err := ReadinessSender(); err == nil {
					t.Fatal("expected error")
				}
			},
		},
		{
			name: "sender success",
			run: func(t *testing.T) {
				t.Helper()
				reader, writer, _ := os.Pipe()
				defer func() {
					err := reader.Close()
					if err != nil {
						t.Fatalf("failed closing pipe reader: %v", err)
					}
				}()

				err := os.Setenv(EnvNameReadinessFD, strconv.Itoa(int(writer.Fd())))
				if err != nil {
					t.Fatalf("unexpected error setting environment variable: %v", err)
				}
				defer func() {
					err := os.Unsetenv(EnvNameReadinessFD)
					if err != nil {
						t.Fatalf("failed unsetting env variable: %v", err)
					}
				}()

				if err := ReadinessSender(); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				buf := make([]byte, len(ReadyMessage))
				if _, err := io.ReadFull(reader, buf); err != nil {
					t.Fatalf("read failed: %v", err)
				}

				if string(buf) != ReadyMessage {
					t.Fatalf("expected %q, got %q", ReadyMessage, buf)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
