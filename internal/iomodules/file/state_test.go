package file

import (
	"os"
	"path/filepath"
	"testing"
)

func setupStateTest(t *testing.T, logContent []byte) (logPath, statePath string) {
	t.Helper()
	tmpDir := t.TempDir()
	logPath = filepath.Join(tmpDir, "log")
	statePath = filepath.Join(tmpDir, "state", "statefile")

	err := os.WriteFile(logPath, logContent, 0o600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}
	return
}

func writeStateRaw(t *testing.T, statePath, data string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(statePath), 0o700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	err = os.WriteFile(statePath, []byte(data), 0o600)
	if err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func assertReset(t *testing.T, logPath, statePath string) {
	t.Helper()
	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}
	if inode != 0 || pos != 0 {
		t.Fatalf("expected reset (0, 0), got (%d, %d)", inode, pos)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected truncated file, got %q", string(data))
	}
}

func TestGetLastPosition_FreshState(t *testing.T) {
	logPath, statePath := setupStateTest(t, []byte("hello"))

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}
	if inode != 0 || pos != 0 {
		t.Fatalf("expected (0, 0), got (%d, %d)", inode, pos)
	}
}

func TestState_RoundTrip(t *testing.T) {
	logPath, statePath := setupStateTest(t, []byte("hello world"))

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	id, err := getFileID(fi)
	if err != nil {
		t.Fatalf("getFileID: %v", err)
	}

	expectedPos := int64(5)
	err = savePosition(statePath, id.ino, expectedPos)
	if err != nil {
		t.Fatalf("savePosition: %v", err)
	}

	for _, label := range []string{"first", "re-save"} {
		t.Run(label, func(t *testing.T) {
			inode, pos, err := getLastPosition(logPath, statePath)
			if err != nil {
				t.Fatalf("getLastPosition: %v", err)
			}
			if inode != id.ino || pos != expectedPos {
				t.Fatalf("expected (%d, %d), got (%d, %d)", id.ino, expectedPos, inode, pos)
			}
			err = savePosition(statePath, inode, pos)
			if err != nil {
				t.Fatalf("savePosition: %v", err)
			}
		})
	}
}

func TestGetLastPosition_InvalidState(t *testing.T) {
	tests := []struct {
		name      string
		stateData string
	}{
		{"garbage data", "invalid data"},
		{"missing value field", "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath, statePath := setupStateTest(t, []byte("hello"))
			writeStateRaw(t, statePath, tt.stateData)
			assertReset(t, logPath, statePath)
		})
	}
}

func TestGetLastPosition_InodeMismatch(t *testing.T) {
	logPath, statePath := setupStateTest(t, []byte("hello"))

	err := savePosition(statePath, 999999, 10)
	if err != nil {
		t.Fatalf("savePosition: %v", err)
	}

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	id, err := getFileID(fi)
	if err != nil {
		t.Fatalf("getFileID: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}
	if inode != id.ino || pos != 0 {
		t.Fatalf("expected (%d, 0), got (%d, %d)", id.ino, inode, pos)
	}
}

func TestGetLastPosition_ClampsToFileSize(t *testing.T) {
	content := []byte("hello")
	logPath, statePath := setupStateTest(t, content)

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	id, err := getFileID(fi)
	if err != nil {
		t.Fatalf("getFileID: %v", err)
	}

	err = savePosition(statePath, id.ino, 9999)
	if err != nil {
		t.Fatalf("savePosition: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}
	expectedPos := int64(len(content))
	if inode != id.ino || pos != expectedPos {
		t.Fatalf("expected (%d, %d), got (%d, %d)", id.ino, expectedPos, inode, pos)
	}
}
