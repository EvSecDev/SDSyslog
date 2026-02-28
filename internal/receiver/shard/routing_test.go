package shard

import (
	"fmt"
	"sdsyslog/internal/crypto/random"
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
			expected:   "s2",
			expectNone: false,
		},
		{
			name:       "multiple candidates deterministic",
			key:        "msg1-4830-3281",
			candidates: []string{"a", "b", "c", "d", "e", "f"},
			expected:   "f",
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

func TestHRWDistribution(t *testing.T) {
	// Mapping: [selectedCandidate]timesSelected
	distribution := make(map[string]int)

	candidateList := []string{"A", "B", "C", "D", "E", "F", "G", "H"}

	sampleSize := 1048576

	// Generate distribution sample
	for i := range sampleSize {
		hostID := i
		msgID, err := random.NumberInRange(0, 65535)
		if err != nil {
			t.Fatalf("unexpected failure getting random data")
		}
		key := fmt.Sprintf("fragment-%d-%d", hostID, msgID)

		selected := hrwSelect(key, candidateList)
		distribution[selected]++
	}

	expectedCandidateShare := 1 / float64(len(candidateList))
	expectedPercent := expectedCandidateShare * 100

	upperDriftLimit := expectedPercent + 1.0
	lowerDriftLimit := expectedPercent - 1.0

	// Validate equal distribution
	for _, candidate := range candidateList {
		count := distribution[candidate]
		gotPercent := float64(count) / float64(sampleSize) * 100

		if gotPercent < lowerDriftLimit || gotPercent > upperDriftLimit {
			t.Errorf("HRW Distribution Abnormality: Candidate=%q ExpectedDistribution=%.2f%% GotDistribution=%.2f%%",
				candidate, expectedPercent, gotPercent)
		}
	}
}
