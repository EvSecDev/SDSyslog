package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetLastPosition_FreshState(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log")
	statePath := filepath.Join(tmpDir, "state", "statefile")

	err := os.WriteFile(logPath, []byte("hello"), 0600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}

	if inode != 0 || pos != 0 {
		t.Fatalf("expected (0 - 0), got (%d - %d)", inode, pos)
	}
}

func TestState_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log")
	statePath := filepath.Join(tmpDir, "state", "statefile")

	err := os.WriteFile(logPath, []byte("hello world"), 0600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}

	id, err := getFileID(fi)
	if err != nil {
		t.Fatalf("failed to get file id: %v", err)
	}
	expectedInode := id.ino
	expectedPos := int64(5)

	err = savePosition(statePath, expectedInode, expectedPos)
	if err != nil {
		t.Fatalf("savePosition: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}

	if inode != expectedInode || pos != expectedPos {
		t.Fatalf("expected (%d - %d), got (%d - %d)", expectedInode, expectedPos, inode, pos)
	}

	// round-trip again
	err = savePosition(statePath, inode, pos)
	if err != nil {
		t.Fatalf("savePosition roundtrip: %v", err)
	}

	inode2, pos2, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition roundtrip: %v", err)
	}

	if inode2 != inode || pos2 != pos {
		t.Fatalf("roundtrip mismatch: (%d - %d) -> (%d - %d)", inode, pos, inode2, pos2)
	}
}

func TestGetLastPosition_InvalidStateTruncates(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log")
	statePath := filepath.Join(tmpDir, "state", "statefile")

	err := os.WriteFile(logPath, []byte("hello"), 0600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(statePath), 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err = os.WriteFile(statePath, []byte("invalid data"), 0600)
	if err != nil {
		t.Fatalf("write state: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}

	if inode != 0 || pos != 0 {
		t.Fatalf("expected reset (0 - 0), got (%d - %d)", inode, pos)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected truncated file, got %q", string(data))
	}
}

func TestGetLastPosition_InodeMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log")
	statePath := filepath.Join(tmpDir, "state", "statefile")

	err := os.WriteFile(logPath, []byte("hello"), 0600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}

	err = savePosition(statePath, 999999, 10)
	if err != nil {
		t.Fatalf("savePosition: %v", err)
	}

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}

	id, err := getFileID(fi)
	if err != nil {
		t.Fatalf("failed to get file id: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("getLastPosition: %v", err)
	}

	if inode != id.ino || pos != 0 {
		t.Fatalf("expected (%d - 0), got (%d - %d)", id.ino, inode, pos)
	}
}

func TestGetLastPosition_ClampsToFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log")
	statePath := filepath.Join(tmpDir, "state", "statefile")

	content := []byte("hello")
	err := os.WriteFile(logPath, content, 0600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat log: %v", err)
	}
	id, err := getFileID(fi)
	if err != nil {
		t.Fatalf("inode get failed: %v", err)
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
		t.Fatalf("expected (%d - %d), got (%d - %d)", id.ino, expectedPos, inode, pos)
	}
}

func TestGetLastPosition_CorruptedStateMissingValue(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "log")
	statePath := filepath.Join(tmpDir, "state", "statefile")

	err := os.WriteFile(logPath, []byte("hello"), 0600)
	if err != nil {
		t.Fatalf("write log: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(statePath), 0700)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// corrupted: only one field
	err = os.WriteFile(statePath, []byte("12345"), 0600)
	if err != nil {
		t.Fatalf("write state: %v", err)
	}

	inode, pos, err := getLastPosition(logPath, statePath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if inode != 0 || pos != 0 {
		t.Fatalf("expected reset (0,0), got (%d,%d)", inode, pos)
	}

	// should be truncated
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected truncated file, got %q", string(data))
	}
}
