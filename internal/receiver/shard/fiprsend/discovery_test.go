package fiprsend

import (
	"net"
	"os"
	"path/filepath"
	"sdsyslog/pkg/fipr"
	"strings"
	"testing"
)

func TestGetSocketFileList(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		selfID       int
		expectedList []string
	}{
		{
			name: "exclude self, normal sorting",
			files: []string{
				fipr.SocketFileNamePrefix + "2" + fipr.SocketFileNameSuffix,
				fipr.SocketFileNamePrefix + "1" + fipr.SocketFileNameSuffix,
				fipr.SocketFileNamePrefix + "3" + fipr.SocketFileNameSuffix,
			},
			selfID: 2,
			expectedList: []string{
				fipr.SocketFileNamePrefix + "1" + fipr.SocketFileNameSuffix,
				fipr.SocketFileNamePrefix + "3" + fipr.SocketFileNameSuffix,
			},
		},
		{
			name:         "no sockets",
			files:        []string{},
			selfID:       1,
			expectedList: []string{},
		},
		{
			name: "non-socket files ignored",
			files: []string{
				"random.txt",
				"socket-999.sock",
				fipr.SocketFileNamePrefix + "5" + fipr.SocketFileNameSuffix,
			},
			selfID: 0,
			expectedList: []string{
				fipr.SocketFileNamePrefix + "5" + fipr.SocketFileNameSuffix,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			for _, f := range tt.files {
				path := filepath.Join(dir, f)

				if strings.HasSuffix(f, fipr.SocketFileNameSuffix) && strings.HasPrefix(f, fipr.SocketFileNamePrefix) {
					// Create an actual UNIX socket
					addr := &net.UnixAddr{Name: path, Net: "unix"}
					l, err := net.ListenUnix("unix", addr)
					if err != nil {
						t.Fatalf("failed to create unix socket: %v", err)
					}
					t.Cleanup(func() { l.Close(); os.Remove(path) })
				} else {
					// non-socket file
					os.WriteFile(path, []byte{}, 0644)
				}
			}

			gotList, err := GetSocketFileList(dir, tt.selfID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(gotList) != len(tt.expectedList) {
				t.Fatalf("expected file list to have %d entries, but it has %d entires", len(tt.expectedList), len(gotList))
			}
			for index, gotEntry := range gotList {
				if tt.expectedList[index] != gotEntry {
					t.Errorf("expected entry at index %d to be '%s', but got '%s'", index, tt.expectedList[index], gotEntry)
				}
			}
		})
	}
}
