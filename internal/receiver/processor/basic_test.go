package processor

import (
	"bytes"
	"context"
	"net/netip"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/internal/tests/utils"
	"sdsyslog/pkg/protocol"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type MockRoutingView struct {
	GetAllIDsFunc            func() []string
	GetNonDrainingIDsFunc    func() []string
	BucketExistsFunc         func(string, string) bool
	GetShardFunc             func(string) *shard.Instance
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

func (m *MockRoutingView) GetShard(shardID string) *shard.Instance {
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

func TestProcessor_Basic(t *testing.T) {
	// Mock program wide encrypt/decrypt
	mockPriv, mockPub, err := ecdh.CreatePersistentKey()
	if err != nil {
		t.Fatalf("unexpected error generating test keys: %v", err)
	}
	err = wrappers.SetupDecryptInnerPayload(mockPriv)
	if err != nil {
		t.Fatalf("unexpected error setting up decryption func: %v", err)
	}
	err = wrappers.SetupEncryptInnerPayload(mockPub)
	if err != nil {
		t.Fatalf("unexpected error setting up encryption func: %v", err)
	}

	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))

	mockMaxPayloadSize := 1320

	mockValidMessage := protocol.Message{
		Timestamp: time.Now(),
		Hostname:  "localhost",
		Fields: map[string]any{
			"test": "msg",
		},
		Data: []byte("hello"),
	}

	tests := []struct {
		name                 string
		input                protocol.Message
		pastCutoffTime       time.Duration
		futureCutoffTime     time.Duration
		expectedMgrError     string
		expectedValidCount   uint64
		expectedErrorMessage string
	}{
		{
			name:               "single packet",
			input:              mockValidMessage,
			pastCutoffTime:     10 * time.Minute,
			futureCutoffTime:   10 * time.Minute,
			expectedValidCount: 1,
		},
		{
			name: "highly fragmented packet",
			input: protocol.Message{
				Timestamp: time.Now(),
				Hostname:  "localhost",
				Fields: map[string]any{
					"a": strings.Repeat("x", 100),
					"b": []byte(strings.Repeat("x", 100)),
				},
				Data: bytes.Repeat([]byte("o"), mockMaxPayloadSize*8),
			},
			pastCutoffTime:     10 * time.Minute,
			futureCutoffTime:   10 * time.Minute,
			expectedValidCount: 11,
		},
		{
			name:             "invalid timestamp window",
			input:            mockValidMessage,
			expectedMgrError: "empty future message cutoff time",
		},
		{
			name: "single packet too far in past",
			input: protocol.Message{
				Timestamp: time.Now().Add(-30 * time.Minute),
				Hostname:  "localhost",
				Fields: map[string]any{
					"test": "msg",
				},
				Data: []byte("hello"),
			},
			pastCutoffTime:       10 * time.Minute,
			futureCutoffTime:     10 * time.Minute,
			expectedValidCount:   0,
			expectedErrorMessage: "has a timestamp too far in the past ",
		},
		{
			name: "single packet too far in future",
			input: protocol.Message{
				Timestamp: time.Now().Add(30 * time.Minute),
				Hostname:  "localhost",
				Fields: map[string]any{
					"test": "msg",
				},
				Data: []byte("hello"),
			},
			pastCutoffTime:       10 * time.Minute,
			futureCutoffTime:     10 * time.Minute,
			expectedValidCount:   0,
			expectedErrorMessage: "has a timestamp too far in the future ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Per test mocks
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = logctx.New(ctx, logctx.NSTest, 1, ctx.Done())

			mockShard := shard.New([]string{logctx.NSTest}, 64, &mockDeadline)
			mockRoutingView := MockRoutingView{
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
				GetShardFunc: func(s string) *shard.Instance {
					return mockShard
				},
			}

			// Mock real packet
			packets, err := protocol.Create(tt.input, 1234, mockMaxPayloadSize, 1, 0)
			if err != nil {
				t.Fatalf("unexpected error from mock packet creation: %v", err)
			}

			// Create a manager obj
			mgrConf := &ManagerConfig{
				MinQueueCapacity: global.DefaultMinQueueSize,
				MaxQueueCapacity: global.DefaultMaxQueueSize,
				PastMsgCutoff:    tt.pastCutoffTime,
				FutureMsgCutoff:  tt.futureCutoffTime,
			}
			mgrConf.MinInstanceCount.Store(1)
			mgrConf.MaxInstanceCount.Store(4)

			procMgr, err := mgrConf.NewManager(ctx, &mockRoutingView)
			gotExpected, err := utils.MatchErrorString(err, tt.expectedMgrError)
			if err != nil {
				t.Fatalf("%v", err)
			} else if gotExpected {
				return
			}

			// Startup an instance
			id := procMgr.AddInstance()

			// Send input to the instance
			for _, packet := range packets {
				container := listener.Container{
					Data: packet,
					Meta: listener.Metadata{
						RemoteIP: netip.MustParseAddr("127.0.0.1"),
					},
				}
				size := len(packet) + 40 // Estimate of overhead of struct plus data
				err := procMgr.Inbox.Push(container, uint64(size))
				if err != nil {
					t.Fatalf("push to processor queue was not successful: %v", err)
				}
			}
			expectedPopCount := len(packets)

			// Wait with timeout until processor consumes input
			maxWaitTime := 500 * time.Millisecond
			endTime := time.Now().Add(maxWaitTime)
			for {
				if time.Now().After(endTime) {
					t.Fatalf("processor instance did not consume %d input(s) after %s", expectedPopCount, maxWaitTime.String())
				}
				actualPopCount := procMgr.Inbox.ActiveWrite.Load().Metrics.PopSuccess.Load()
				if actualPopCount == uint64(expectedPopCount) {
					break
				}
			}

			instanceList := *procMgr.Instances.Load()
			instance := instanceList[0]

			// Shutdown instance
			removedID := procMgr.RemoveLastInstance()
			if id != removedID {
				t.Errorf("expected instance removal to remove id %d, but it removed id %d", id, removedID)
			}

			// We hold the pointer from before removal, so we can collect metrics after worker is fully shutdown
			metrics := instance.CollectMetrics(1 * time.Second)

			// Validate metrics from the collection func point of view
			for _, metric := range metrics {
				value := metric.Value.Raw.(uint64)
				if metric.Name == MTValidPayloads && value != tt.expectedValidCount {
					t.Errorf("expected metric valid payloads count to be %d, but got %d", tt.expectedValidCount, value)
				}
				if metric.Name == MTMaxWorkTime && value <= 0 {
					t.Errorf("expected metric elapsed max work time to be greater than zero, but got %d", value)
				}
				if metric.Name == MTSumWorkTime && value <= 0 {
					t.Errorf("expected metric elapsed sum work time to be greater than zero, but got %d", value)
				}
			}

			// Validate no errors in log
			_, err = utils.MatchLogCtxErrors(ctx, tt.expectedErrorMessage, nil)
			if err != nil {
				t.Fatalf("%v", err)
			}
		})
	}
}
