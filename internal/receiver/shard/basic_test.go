package shard

import (
	"context"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/global"
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
		name          string
		fragments     []protocol.Payload
		expectedQueue int
		expectedBytes uint64
	}{
		{
			name: "single fragment completes bucket",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, LogText: []byte("A")},
			},
			expectedQueue: 1,
			expectedBytes: 1,
		},
		{
			name: "multi-fragment fills bucket in order",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 1, LogText: []byte("A")},
				{HostID: 1, MessageSeq: 1, MessageSeqMax: 1, LogText: []byte("B")},
			},
			expectedQueue: 1,
			expectedBytes: 2,
		},
		{
			name: "out-of-order fragments",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 1, MessageSeqMax: 1, LogText: []byte("B")},
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 1, LogText: []byte("A")},
			},
			expectedQueue: 1,
			expectedBytes: 2,
		},
		{
			name: "duplicate fragments ignored",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, LogText: []byte("A")},
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, LogText: []byte("A")},
			},
			expectedQueue: 1,
			expectedBytes: 1,
		},
		{
			name: "timeout triggers bucket fill",
			fragments: []protocol.Payload{
				{HostID: 1, MessageSeq: 0, MessageSeqMax: 2, LogText: []byte("A")},
			},
			expectedQueue: 1,
			expectedBytes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := New([]string{global.NSTest}, 10, &mockDeadline)

			// inject fragments
			for _, frag := range tt.fragments {
				queue.push(mockCtx, "bucket1", frag, time.Now())
			}

			// simulate timeout
			if tt.name == "timeout triggers bucket fill" {
				time.Sleep(60 * time.Millisecond)
				go queue.StartTimeoutWatcher(mockCtx)
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
			if got := len(bucket.Fragments); uint64(got) != tt.expectedBytes {
				t.Fatalf("expected %d bytes/fragments, got %d", tt.expectedBytes, got)
			}

			// metrics sanity
			if queue.Metrics.TotalBuckets.Load() != 0 {
				t.Fatalf("total buckets should be 0 after pop, got %d", queue.Metrics.TotalBuckets.Load())
			}
			if queue.Metrics.WaitingBuckets.Load() != 0 {
				t.Fatalf("waiting buckets should be 0 after pop, got %d", queue.Metrics.WaitingBuckets.Load())
			}
		})
	}
}

// Parallel producer test to ensure concurrency safety
func TestPushPop_Parallel(t *testing.T) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))

	queue := New([]string{global.NSTest}, 10, &mockDeadline)
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
					LogText:       []byte("data"),
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
