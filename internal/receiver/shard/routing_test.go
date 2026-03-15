package shard

import (
	"context"
	"fmt"
	"net/netip"
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
	SocketDirFunc            func() string
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

func (m *MockRoutingView) SocketDir() string {
	return m.SocketDirFunc()
}

func TestRouteFragment(t *testing.T) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))

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

	var existingFragmentFlag *bool

	tests := []struct {
		name                 string
		mockRV               MockRoutingView
		remoteAddr           string
		inputFrags           []protocol.Payload
		processStartTime     time.Time
		expectedRouteSuccess bool
		expectedShardIDDest  string
		expectedErrorLog     string
	}{
		{
			name:                 "Local new fragment",
			remoteAddr:           "127.0.0.1",
			inputFrags:           []protocol.Payload{mockFrag},
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
					return false
				},
				GetNonDrainingIDsFunc: func() []string {
					return []string{"s1", "s2"}
				},
				GetShardFunc: func(s string) *Instance {
					if s == "s2" {
						return mockShard
					}
					return nil
				},
			},
		},
		{
			name:                 "Local new fragment single shard",
			remoteAddr:           "127.0.0.1",
			inputFrags:           []protocol.Payload{mockFrag},
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
				GetNonDrainingIDsFunc: func() []string {
					return []string{"s1"}
				},
				GetShardFunc: func(s string) *Instance {
					if s == "s1" {
						return mockShard
					}
					return nil
				},
			},
		},
		{
			name:       "Local existing fragment",
			remoteAddr: "127.0.0.1",
			inputFrags: []protocol.Payload{
				{
					HostID:        1234,
					MsgID:         4321,
					MessageSeq:    0,
					MessageSeqMax: 1,
					Timestamp:     time.Now(),
					Hostname:      "localhost",
					Data:          []byte("A"),
				},
				{
					HostID:        1234,
					MsgID:         4321,
					MessageSeq:    1,
					MessageSeqMax: 1,
					Timestamp:     time.Now(),
					Hostname:      "localhost",
					Data:          []byte("B"),
				},
			},
			processStartTime:     time.Now(),
			expectedShardIDDest:  "s1",
			expectedRouteSuccess: true,
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{"s1", "s2", "s3"}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				BucketExistsFunc: func(shardID, bucketKey string) bool {
					if shardID == "s1" {
						if existingFragmentFlag == nil {
							newFlag := true
							existingFragmentFlag = &newFlag
							return false
						} else {
							return true
						}
					}
					return false
				},
				IsShardShutdownFunc: func(s string) bool {
					return false
				},
				GetNonDrainingIDsFunc: func() []string {
					return []string{"s1", "s2", "s3"}
				},
				GetShardFunc: func(s string) *Instance {
					if s == "s1" {
						return mockShard
					}
					return nil
				},
			},
		},
		{
			name:       "Local existing fragment - default shard shutdown",
			remoteAddr: "127.0.0.1",
			inputFrags: []protocol.Payload{
				{
					HostID:        1234,
					MsgID:         5783,
					MessageSeq:    0,
					MessageSeqMax: 1,
					Timestamp:     time.Now(),
					Hostname:      "localhost",
					Data:          []byte("A"),
				},
				{
					HostID:        1234,
					MsgID:         5783,
					MessageSeq:    1,
					MessageSeqMax: 1,
					Timestamp:     time.Now(),
					Hostname:      "localhost",
					Data:          []byte("B"),
				},
			},
			processStartTime:     time.Now(),
			expectedRouteSuccess: true,
			expectedShardIDDest:  "s2",
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{"s1", "s2", "s3", "s4"}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				BucketExistsFunc: func(shardID, bucketKey string) bool {
					if shardID == "s2" {
						if existingFragmentFlag == nil {
							newFlag := true
							existingFragmentFlag = &newFlag
							return false
						} else {
							return true
						}
					}
					return false
				},
				IsShardShutdownFunc: func(s string) bool {
					if s == "s2" {
						return true
					} else {
						return false
					}
				},
				GetNonDrainingIDsFunc: func() []string {
					return []string{"s1", "s2", "s3", "s4"}
				},
				GetShardFunc: func(s string) *Instance {
					if s == "s2" || s == "s3" {
						return mockShard
					}
					return nil
				},
			},
		},
		{
			name:                 "Empty shard list",
			inputFrags:           []protocol.Payload{mockFrag},
			expectedErrorLog:     "no active shards available",
			expectedRouteSuccess: false,
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string {
					return []string{}
				},
				IsFIPRRunningFunc: func() bool {
					return false
				},
				GetNonDrainingIDsFunc: func() []string {
					return []string{}
				},
				GetShardFunc: func(s string) *Instance {
					return nil
				},
			},
		},
		{
			name:       "Retry limit hit",
			remoteAddr: "127.0.0.1",
			inputFrags: []protocol.Payload{
				{
					HostID:        1234,
					MsgID:         5783,
					MessageSeq:    0,
					MessageSeqMax: 1,
					Timestamp:     time.Now(),
					Hostname:      "localhost",
					Data:          []byte("A"),
				},
				{
					HostID:        1234,
					MsgID:         5783,
					MessageSeq:    1,
					MessageSeqMax: 1,
					Timestamp:     time.Now(),
					Hostname:      "localhost",
					Data:          []byte("B"),
				},
			},
			expectedRouteSuccess: false,
			expectedErrorLog:     "Dropped message ID",
			mockRV: MockRoutingView{
				GetAllIDsFunc: func() []string { return []string{"s1"} },
				BucketExistsFunc: func(shardID, bucketKey string) bool {
					return false
				},
				IsFIPRRunningFunc:   func() bool { return false },
				IsShardShutdownFunc: func(s string) bool { return true },
				GetNonDrainingIDsFunc: func() []string {
					return []string{}
				},
				GetShardFunc: func(s string) *Instance { return nil },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure flag is per-test only
			existingFragmentFlag = nil

			baseCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mockCtx := logctx.New(baseCtx, "test", logctx.VerbosityFullData, baseCtx.Done())

			mockShard = New([]string{logctx.NSTest}, 64, &mockDeadline)
			defer func() {
				mockShard = nil
			}()
			if mockShard == nil {
				t.Fatalf("mock shard not initialized`")
			}
			logger := logctx.GetLogger(mockCtx)

			var remoteAddr netip.Addr
			var err error
			if tt.remoteAddr != "" {
				remoteAddr, err = netip.ParseAddr(tt.remoteAddr)
				if err != nil {
					t.Fatalf("unexpected parsing error: %v", err)
				}
			}

			for _, fragment := range tt.inputFrags {
				routeSuccess := RouteFragment(mockCtx, &tt.mockRV, remoteAddr, fragment, tt.processStartTime)
				allLogLines := logger.GetFormattedLogLines()
				var foundMatchingExpectedError bool
				for _, line := range allLogLines {
					if !strings.Contains(line, "["+logctx.ErrorLog+"]") && !strings.Contains(line, "["+logctx.WarnLog+"]") {
						continue
					}
					// Test is expecting error
					if tt.expectedErrorLog == "" {
						t.Errorf("expected no error in logs, but found: %q", line)
						continue
					}

					if strings.Contains(line, tt.expectedErrorLog) {
						foundMatchingExpectedError = true
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
			if bucket.maxSeq != tt.inputFrags[0].MessageSeqMax {
				t.Fatalf("expected bucket maximum sequence to be %d, but got %d", tt.inputFrags[0].MessageSeqMax, bucket.maxSeq)
			}
			if bucket.filled != true {
				t.Fatalf("expected bucket to be marked as filled, but it was not")
			}

			if len(bucket.Fragments) != len(tt.inputFrags) {
				t.Fatalf("expected bucket fragment map to contain %d entries, got %d",
					len(tt.inputFrags), len(bucket.Fragments))
			}
			for _, input := range tt.inputFrags {
				fragment, ok := bucket.Fragments[input.MessageSeq]
				if !ok {
					t.Fatalf("missing fragment seq %d", input.MessageSeq)
				}

				if !input.EqualTo(fragment) {
					t.Fatalf("fragment mismatch\ninput: %+v\noutput: %+v", input, fragment)
				}
			}
		})
	}
}

func TestRouteSelect(t *testing.T) {
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
			expected:   "s1",
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
				selectedSecond, _ := routeSelect(tt.key, tt.candidates)
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

func TestRouteDistribution(t *testing.T) {
	sampleSize := 1048576

	for size := 2; size <= 128; size *= 2 {
		// Mapping: [selectedCandidate]timesSelected
		distribution := make(map[string]int)

		candidateList := make([]string, size)
		for i := 0; i < size; i++ {
			candidateList[i] = fmt.Sprintf("A-%d", i)
		}

		// Generate distribution sample
		for i := range sampleSize {
			hostID := i
			msgID, err := random.NumberInRange(0, 65535)
			if err != nil {
				t.Fatalf("unexpected failure getting random data")
			}
			key := fmt.Sprintf("fragment-%d-%d", hostID, msgID)

			selected, _ := routeSelect(key, candidateList)
			distribution[selected]++
		}

		expectedCandidateShare := 1.0 / float64(len(candidateList))
		expectedPercent := expectedCandidateShare * 100

		drift := 0.25
		upperDriftLimit := expectedPercent + drift
		lowerDriftLimit := expectedPercent - drift

		// Validate equal distribution
		for _, candidate := range candidateList {
			count := distribution[candidate]
			gotPercent := float64(count) / float64(sampleSize) * 100

			if gotPercent < lowerDriftLimit || gotPercent > upperDriftLimit {
				t.Errorf("Route Distribution Abnormality (Candidate Count %d): Candidate=%q ExpectedDistribution=%.2f%% GotDistribution=%.2f%%",
					size, candidate, expectedPercent, gotPercent)
			}
		}
	}
}

func TestRoutePerformance(t *testing.T) {
	key := "test-key"
	var prevNsPerOp float64

	for size := 2; size <= 128; size *= 2 {
		candidates := make([]string, size)
		for i := 0; i < size; i++ {
			candidates[i] = fmt.Sprintf("A-%d", i)
		}

		b := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				routeSelect(key, candidates)
			}
		})

		nsPerOp := float64(b.NsPerOp())
		allocsPerOp := b.AllocsPerOp()
		bytesPerOp := b.AllocedBytesPerOp()

		t.Logf("size=%3d: ns/op=%.2f, allocs/op=%d, bytes/op=%.2f",
			size, nsPerOp, allocsPerOp, float64(bytesPerOp))

		// Assert: no sudden ns/op increase
		if prevNsPerOp > 0 && nsPerOp > prevNsPerOp*2 {
			t.Fatalf("hrwSelect ns/op increased too much: prev=%.2f, current=%.2f",
				prevNsPerOp, nsPerOp)
		}
		prevNsPerOp = nsPerOp

		// Assert: no allocations
		if allocsPerOp != 0 {
			t.Fatalf("hrwSelect allocations detected for size=%d: %d allocs/op", size, allocsPerOp)
		}

		// Assert: no memory usage
		if bytesPerOp != 0 {
			t.Fatalf("hrwSelect memory allocated for size=%d: %.2f B/op", size, float64(bytesPerOp))
		}
	}
}
