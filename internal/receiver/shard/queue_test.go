package shard

import (
	"context"
	"fmt"
	"sdsyslog/internal/logctx"
	"sdsyslog/pkg/protocol"
	"sync/atomic"
	"testing"
	"time"
)

func TestPushDoesNotHoldLockWhenQueueFull(t *testing.T) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(50 * int64(time.Millisecond))

	const keyQueueCap = 2
	queue := New([]string{logctx.NSTest}, keyQueueCap, &mockDeadline)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Fill the key queue to capacity so the next enqueue blocks
	for index := range keyQueueCap {
		queue.keyQueue <- fmt.Sprintf("seed-%d", index)
	}

	// Hold the lock so the push below queues behind it.
	// Then release it so the push is guaranteed to acquire the lock and reach the (full) blocking enqueue.
	queue.Mu.Lock()
	pushDone := make(chan struct{})
	go func() {
		defer close(pushDone)
		frag := &protocol.Payload{HostID: 1, MessageSeq: 0, MessageSeqMax: 0, Data: []byte("A")}
		queue.push(ctx, "blocked-bucket", frag, time.Now())
	}()
	queue.Mu.Unlock()

	// Give the push a moment to acquire the lock and reach the blocked enqueue.
	// The key queue is full, so the push is guaranteed to block there.
	time.Sleep(50 * time.Millisecond)

	// While the push is blocked on the full key queue the shard lock must be free,
	// otherwise the assembler cannot drain the queue and the shard deadlocks.
	lockFree := make(chan struct{})
	go func() {
		queue.Mu.Lock()
		defer queue.Mu.Unlock()
		close(lockFree)
	}()

	select {
	case <-lockFree:
		// Lock was free while the push was blocked. Free a slot so the blocked
		// push can complete and its goroutine exits.
		if _, ok := queue.PopKey(ctx); !ok {
			t.Fatal("expected to free a keyQueue slot for the blocked push")
		}
	case <-ctx.Done():
		t.Fatal("push held the shard lock while blocked on a full keyQueue")
	}

	// Ensure the blocked push actually completed once the slot was freed.
	select {
	case <-pushDone:
	case <-ctx.Done():
		t.Fatal("push did not complete after a keyQueue slot was freed")
	}
}
