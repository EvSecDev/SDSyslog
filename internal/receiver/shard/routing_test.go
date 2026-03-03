package shard

import (
	"context"
	"fmt"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type MockRoutingView struct {
	GetAllIDsFunc            func() []string
	GetNonDrainingIDsFunc    func() []string
	BucketExistsFunc         func(string, string) bool
	GetShardFunc             func(string) *Instance
	IsShardShutdownFunc      func(string) bool
	BucketExistsAnywhereFunc func(string) bool
	IsFIPRRunningFunc        func() bool
}

func (m *MockRoutingView) GetAllIDs() []string {
	return m.GetAllIDsFunc()
}

func (m *MockRoutingView) GetNonDrainingIDs() []string {
	return m.GetNonDrainingIDsFunc()
}

func (m *MockRoutingView) BucketExists(shardID, bucketKey string) bool {
	return m.BucketExistsFunc(shardID, bucketKey)
}

func (m *MockRoutingView) GetShard(shardID string) *Instance {
	return m.GetShardFunc(shardID)
}

func (m *MockRoutingView) IsShardShutdown(shardID string) bool {
	return m.IsShardShutdownFunc(shardID)
}

func (m *MockRoutingView) BucketExistsAnywhere(bucketKey string) bool {
	return m.BucketExistsAnywhereFunc(bucketKey)
}

func (m *MockRoutingView) IsFIPRRunning() bool {
	return m.IsFIPRRunningFunc()
}

func TestRouteFragment(t *testing.T) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))
	baseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mockCtx := logctx.New(baseCtx, "test", logctx.VerbosityFullData, baseCtx.Done())

	mockShard := &Instance{}

	mockFrag := protocol.Payload{
		HostID:        1234,
		MsgID:         4567,
		MessageSeq:    0,
		MessageSeqMax: 0,
		Timestamp:     time.Now(),
		Hostname:      "localhost",
		Data:          []byte("A"),
	}

	tests := []struct {
		name                 string
		mockRV               MockRoutingView
		remoteAddr           string
		inputFrag            protocol.Payload
		processStartTime     time.Time
		expectedRouteSuccess bool
		expectedShardIDDest  string
		expectedErrorLog     string
	}{
		{
			name:                 "Local new fragment",
			remoteAddr:           "127.0.0.1",
			inputFrag:            mockFrag,
			processStartTime:     time.Now(),
			expectedRouteSuccess: true,
			expectedShardIDDest:  "s1",
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{"s1"}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				BucketExistsFunc: func(shardID, bucketKey string) bool {
					return false
				},
				IsShardShutdownFunc: func(s string) bool {
					return false
				},
				GetShardFunc: func(s string) *Instance {
					return mockShard
				},
			},
		},
		{
			name:                 "Local existing fragment",
			remoteAddr:           "127.0.0.1",
			inputFrag:            mockFrag,
			processStartTime:     time.Now(),
			expectedShardIDDest:  "s1",
			expectedRouteSuccess: true,
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{"s1"}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				BucketExistsFunc: func(shardID, bucketKey string) bool {
					return true
				},
				IsShardShutdownFunc: func(s string) bool {
					return false
				},
				GetShardFunc: func(s string) *Instance {
					return mockShard
				},
			},
		},
		{
			name:                 "Local existing fragment - default shard shutdown",
			remoteAddr:           "127.0.0.1",
			inputFrag:            mockFrag,
			processStartTime:     time.Now(),
			expectedRouteSuccess: true,
			expectedShardIDDest:  "s2",
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{"s1", "s2"}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				BucketExistsFunc: func(shardID, bucketKey string) bool {
					return false
				},
				IsShardShutdownFunc: func(s string) bool {
					if s == "s1" {
						return true
					} else {
						return false
					}
				},
				GetNonDrainingIDsFunc: func() []string {
					return []string{"s2"}
				},
				GetShardFunc: func(s string) *Instance {
					return mockShard
				},
			},
		},
		{
			name:                 "Empty shard list",
			expectedErrorLog:     "no shards available",
			expectedRouteSuccess: false,
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				GetShardFunc: func(s string) *Instance {
					return mockShard
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockShard = New([]string{logctx.NSTest}, 64, &mockDeadline)
			defer func() {
				mockShard = nil
			}()
			if mockShard == nil {
				t.Fatalf("mock shard not initialized`")
			}

			routeSuccess := RouteFragment(mockCtx, &tt.mockRV, tt.remoteAddr, tt.inputFrag, tt.processStartTime)
			logger := logctx.GetLogger(mockCtx)
			allLogLines := logger.GetFormattedLogLines()
			var foundMatchingExpectedError bool
			for _, line := range allLogLines {
				if !strings.Contains(line, "["+logctx.ErrorLog+"]") && !strings.Contains(line, "["+logctx.WarnLog+"]") {
					continue
				}
				// Test is expecting error
				if tt.expectedErrorLog != "" && strings.Contains(line, tt.expectedErrorLog) {
					foundMatchingExpectedError = true
					break
				} else {
					t.Errorf("expected no error in logs, but found: %q", line)
				}
			}
			if tt.expectedErrorLog != "" && !foundMatchingExpectedError {
				t.Fatalf("expected error log containing %q, but found none", tt.expectedErrorLog)
			} else if tt.expectedErrorLog != "" && foundMatchingExpectedError {
				return // No additional test evaluations for this run
			}

			if routeSuccess != tt.expectedRouteSuccess {
				t.Fatalf("expected route success=%v, but got success=%v", tt.expectedRouteSuccess, routeSuccess)
			}

			expectedShard := tt.mockRV.GetShardFunc(tt.expectedShardIDDest)
			if expectedShard == nil {
				t.Fatalf("expected shard to be id %s, but no shard exists at that id", tt.expectedShardIDDest)
			}

			key, ok := expectedShard.PopKey(mockCtx)
			if !ok {
				t.Fatalf("unexpected shard pop failure: bucket key retrieval returned false for success")
			}

			bucket, notExists := expectedShard.DrainBucket(mockCtx, key)
			if notExists {
				t.Fatalf("unexpected shard bucket retrieval failure: shard reports bucket does not exist")
			}
			if bucket == nil {
				t.Fatalf("unexpected shard bucket retrieval failure: retrieved bucket is nil")
			}

			if bucket.lastProcessStartTime != tt.processStartTime {
				t.Fatalf("expected bucket last processing time to be '%v', but it is actually '%v'", tt.processStartTime, bucket.lastProcessStartTime)
			}
			if bucket.maxSeq != tt.inputFrag.MessageSeqMax {
				t.Fatalf("expected bucket maximum sequence to be %d, but got %d", tt.inputFrag.MessageSeqMax, bucket.maxSeq)
			}
			if bucket.filled != true {
				t.Fatalf("expected bucket to be marked as filled, but it was not")
			}

			if len(bucket.Fragments) != 1 {
				t.Fatalf("expected bucket fragment map to contain 1 entry, but got %d entry(ies)", len(bucket.Fragments))
			}
			for id, fragment := range bucket.Fragments {
				if id != tt.inputFrag.MessageSeq {
					t.Fatalf("expected output fragment id to be %d, but got %d", tt.inputFrag.MessageSeq, id)
				}
				if !tt.inputFrag.EqualTo(fragment) {
					t.Fatalf("expected input fragment to be the same as output fragment.\nInput: %+v\nOutput:%+v\n", tt.inputFrag, fragment)
				}
			}
		})
	}
}

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
