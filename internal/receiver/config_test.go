package receiver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sdsyslog/internal/crypto/wrappers"
	"slices"
	"strings"
	"testing"
)

func TestReloadSigningKeys(t *testing.T) {
	tests := []struct {
		name              string
		pinnedKeysJSON    map[string][]byte
		newPinnedKeysJSON string
		pinnedKeyFile     string
		expectedDiffCount int
		expectedError     string
	}{
		{
			name: "No change",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
			},
			newPinnedKeysJSON: `{"host1": "YWFhYQ=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 0,
		},
		{
			name: "Single new",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
			},
			newPinnedKeysJSON: `{"host1": "YWFhYQ==", "host2": "YmJiYg=="}`, // JSON marshalling encodes bytes as base64
			pinnedKeyFile:     "pins",
			expectedDiffCount: 1,
		},
		{
			name: "Single delete",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
				"host2": []byte("bbbb"),
			},
			newPinnedKeysJSON: `{"host1": "YWFhYQ=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 1,
		},
		{
			name: "Single key change",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
			},
			newPinnedKeysJSON: `{"host1": "YmJiYg=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 1,
		},
		{
			name: "Delete all",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
			},
			newPinnedKeysJSON: `{}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 1,
		},
		{
			name:              "Brand new",
			pinnedKeysJSON:    map[string][]byte{},
			newPinnedKeysJSON: `{"host1": "YWFhYQ=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 1,
		},
		{
			name: "Delete many",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
				"host2": []byte("bbbb"),
				"host3": []byte("cccc"),
				"host4": []byte("dddd"),
			},
			newPinnedKeysJSON: `{"host1": "YWFhYQ=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 3,
		},
		{
			name: "Multiple - change, add, delete",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
				"host2": []byte("bbbb"),
				"host3": []byte("cccc"),
				"host4": []byte("dddd"),
			},
			newPinnedKeysJSON: `{"host1": "YWFhYQ==", "host5": "ZWVlZQ==", "host2": "YmJiYg==", "host3": "Y2NmZg=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 3,
		},
		{
			name: "No pinned keys path",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
				"host2": []byte("bbbb"),
			},
			pinnedKeyFile:     "",
			expectedDiffCount: 0,
		},
		{
			name: "Empty file",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
			},
			newPinnedKeysJSON: ``,
			pinnedKeyFile:     "pins",
			expectedError:     "failed to parse pinned keys JSON",
		},
		{
			name: "Invalid New JSON",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
			},
			newPinnedKeysJSON: `{"host1": invalid}`,
			pinnedKeyFile:     "pins",
			expectedError:     "failed to parse pinned keys JSON",
		},
		{
			name: "Change and delete different keys",
			pinnedKeysJSON: map[string][]byte{
				"host1": []byte("aaaa"),
				"host2": []byte("bbbb"),
			},
			newPinnedKeysJSON: `{"host1": "YmJiYg=="}`,
			pinnedKeyFile:     "pins",
			expectedDiffCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mocks

			var mockDaemon Daemon
			mockDaemon.cfg.PinnedSigningKeys = tt.pinnedKeysJSON
			wrappers.NewPinnedSenders(mockDaemon.cfg.PinnedSigningKeys)

			testDir := t.TempDir()
			mockDaemon.cfg.path = filepath.Join(testDir, "main.json")

			var pinnedKeyPath string
			if tt.pinnedKeyFile != "" {
				pinnedKeyPath = filepath.Join(testDir, tt.pinnedKeyFile)
				err := os.WriteFile(pinnedKeyPath, []byte(tt.newPinnedKeysJSON), 0644)
				if err != nil {
					t.Fatalf("failed to write test pinned keys JSON: %v", err)
				}
			}

			var mockDaemonConfig JSONConfig
			mockDaemonConfig.PinnedSigningKeysPath = pinnedKeyPath
			mockDaemon.cfg.PinnedSigningKeysFile = pinnedKeyPath

			cfg, err := json.Marshal(&mockDaemonConfig)
			if err != nil {
				t.Fatalf("failed to marshal mock daemon JSON: %v", err)
			}
			err = os.WriteFile(mockDaemon.cfg.path, cfg, 0644)
			if err != nil {
				t.Fatalf("failed to write mock daemon JSON: %v", err)
			}

			// Call reload
			diffCount, err := mockDaemon.ReloadSigningKeys()

			// Verify error state
			if err != nil && tt.expectedError == "" {
				t.Fatalf("unexpected error from reload: %v", err)
			}
			if err == nil && tt.expectedError != "" {
				t.Fatalf("expected error %q, but got none", tt.expectedError)
			}
			if err != nil && !strings.Contains(err.Error(), tt.expectedError) {
				t.Fatalf("expected error %q, but got error %v", tt.expectedError, err)
			}
			if err != nil && strings.Contains(err.Error(), tt.expectedError) {
				return
			}

			// Verify count is accurate
			if diffCount != tt.expectedDiffCount {
				t.Errorf("expected diff count %d, but got diff count %d", tt.expectedDiffCount, diffCount)
			}

			// Verify keys were loaded and/or untouched
			for host, key := range mockDaemon.cfg.PinnedSigningKeys {
				gotKey, present := wrappers.LookupPinnedSender(host)
				if !present {
					t.Errorf("expected host %q to have key %q present in loaded map, but it was not present", host, string(key))
					continue
				}
				if !slices.Equal(key, gotKey) {
					t.Errorf("host %q: expected key to be %q, but got %q", host, string(key), string(gotKey))
				}
			}
		})
	}
}
