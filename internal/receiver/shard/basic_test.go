package shard

import (
	"context"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Basic cases covering expected code paths
func TestPushPop_Basic(t *testing.T) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))
	mockCtx := context.Background()

	tests := []struct {
		name                   string
		fragments              []protocol.Payload
		pushDelay              time.Duration // Time between fragment push-to-queue
		expectedTimeOutBuckets int
		expectedBytes          uint64
	}{
		{
			name: "single fragment completes bucket",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, Data: []byte("A")},
			},
			expectedBytes: 1,
		},
		{
			name: "multi-fragment fills bucket in order",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 1, Data: []byte("A")},
				{HostID: 1, MessageSeq: 1, MessageSeqMax: 1, Data: []byte("B")},
			},
			expectedBytes: 2,
		},
		{
			name: "out-of-order fragments",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 1, MessageSeqMax: 1, Data: []byte("B")},
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 1, Data: []byte("A")},
			},
			expectedBytes: 2,
		},
		{
			name: "duplicate fragments ignored",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, Data: []byte("A")},
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, Data: []byte("A")},
			},
			expectedBytes: 1,
		},
		{
			name: "timeout triggers bucket fill",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 2, Data: []byte("A")},
			},
			expectedTimeOutBuckets: 1,
			expectedBytes:          1,
		},
		{
			name: "timeout in hot path",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 1, Data: []byte("A")},
				{HostID: 1, MessageSeq: 1, MessageSeqMax: 1, Data: []byte("fake")},
			},
			pushDelay:              time.Duration(mockDeadline.Load()) + 10*time.Millisecond,
			expectedTimeOutBuckets: 1,
			expectedBytes:          1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := New([]string{logctx.NSTest}, 10, &mockDeadline)
			if tt.name == "timeout triggers bucket fill" {
				go queue.StartTimeoutWatcher(mockCtx)
			}

			// Output count is expected post-defragmentation count
			seenMsgIds := make(map[int]struct{})
			for _, frag := range tt.fragments {
				seenMsgIds[frag.MsgID] = struct{}{}
			}
			expectedMsgCount := len(seenMsgIds)

			// inject fragments
			for _, frag := range tt.fragments {
				queue.push(mockCtx, "bucket1", frag, time.Now())
				if tt.pushDelay != 0 {
					time.Sleep(tt.pushDelay)
				}
			}

			// pop bucket
			key, ok := queue.PopKey(mockCtx)
			if !ok {
				t.Fatalf("expected to pop a key, got none")
			}
			if key != "bucket1" {
				t.Fatalf("expected bucket1 key, got %s", key)
			}

			bucket, notExist := queue.DrainBucket(mockCtx, key)
			if notExist {
				t.Fatalf("bucket should exist")
			}

			// validate fragments count
			got := len(bucket.Fragments)
			if uint64(got) != tt.expectedBytes {
				t.Fatalf("expected %d bytes/fragments, got %d", tt.expectedBytes, got)
			}

			metrics := queue.CollectMetrics(1 * time.Minute)

			// Validate metrics from the collection func point of view
			for _, metric := range metrics {
				value := metric.Value.Raw.(uint64)
				if metric.Name == MTPopCnt && int(value) != expectedMsgCount {
					t.Errorf("expected metric pop count to be %d, but got %d", expectedMsgCount, value)
				}
				if metric.Name == MTPushCnt && int(value) != len(tt.fragments) {
					t.Errorf("expected metric push count to be %d, but got %d", len(tt.fragments), value)
				}
				if metric.Name == MTTotalBuckets && value != 0 {
					t.Errorf("expected metric total bucket count to be 0 after test, but got %d", value)
				}
				if metric.Name == MTWaitingBuckets && value != 0 {
					t.Errorf("expected metric waiting bucket count to be 0 after test, but got %d", value)
				}
				if metric.Name == MTTimedOutBuckets {
					if value != uint64(tt.expectedTimeOutBuckets) {
						t.Errorf("expected metric timed out buckets value to be %d, but got %d", tt.expectedTimeOutBuckets, value)
					}
				}
				if metric.Name == MTTimeBtwFragments && value == 0 {
					t.Errorf("expected metric time between fragments to be above 0")
				}
				if metric.Name == MTBytes && value != 0 {
					t.Errorf("expected metric total bytes to be 0 after tests, but got %d bytes", value)
				}
			}
		})
	}
}

// Parallel producer test to ensure concurrency safety
func TestPushPop_Parallel(t *testing.T) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))

	queue := New([]string{logctx.NSTest}, 10, &mockDeadline)
	ctx := context.Background()
	numProducers := 3
	fragsPerProducer := 5
	var wg sync.WaitGroup

	for p := 0; p < numProducers; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()

			uniqueID, _ := random.FourByte()

			for i := 0; i < fragsPerProducer; i++ {
				frag := protocol.Payload{
					HostID:        pid,
					MessageSeq:    i,
					MessageSeqMax: fragsPerProducer - 1,
					Data:          []byte("data"),
				}
				key := "bucket" + strconv.Itoa(uniqueID+fragsPerProducer)
				queue.push(ctx, key, frag, time.Now())
			}
		}(p)
	}

	wg.Wait()

	for p := 0; p < numProducers; p++ {
		key, ok := queue.PopKey(ctx)
		if !ok {
			t.Fatalf("expected to pop a key for producer %d", p)
		}
		bucket, notExist := queue.DrainBucket(ctx, key)
		if notExist {
			t.Fatalf("bucket should exist for key %s", key)
		}

		if len(bucket.Fragments) != fragsPerProducer {
			t.Fatalf("expected %d fragments in bucket '%s', got %d", fragsPerProducer, key, len(bucket.Fragments))
		}
	}
}
