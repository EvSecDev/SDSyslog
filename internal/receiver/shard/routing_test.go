package shard

import (
	"slices"
	"testing"
)

func TestHRWSelect(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		candidates []string
		expectNone bool
		expected   string
	}{
		{
			name:       "single candidate",
			key:        "msg1",
			candidates: []string{"shardA"},
			expectNone: false,
			expected:   "shardA",
		},
		{
			name:       "two candidates deterministic",
			key:        "msg1",
			candidates: []string{"shardA", "shardB"},
			expected:   "shardA",
			expectNone: false,
		},
		{
			name:       "multiple candidates deterministic",
			key:        "msg2",
			candidates: []string{"s1", "s2", "s3", "s4"},
			expected:   "s4",
			expectNone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Deterministic check: repeat call should give same result
			for i := 0; i < 10; i++ {
				selectedSecond := hrwSelect(tt.key, tt.candidates)
				if !slices.Contains(tt.candidates, selectedSecond) {
					t.Fatalf("selected value '%s' is not in test candidates %q", selectedSecond, tt.candidates)
				}
				if selectedSecond != tt.expected {
					t.Fatalf("expected selected to be '%s', but got '%s'", tt.expected, selectedSecond)
				}
			}
		})
	}
}
