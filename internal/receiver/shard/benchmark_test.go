package shard

import (
	"bytes"
	"context"
	"fmt"
	"sdsyslog/internal/global"
	"sdsyslog/pkg/protocol"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkQueue_Scaling(b *testing.B) {
	var mockDeadline atomic.Int64
	mockDeadline.Store(int64(100))
	mockCtx := context.Background()

	payloadTemplate := protocol.Payload{
		HostID:          101,
		LogID:           202,
		MessageSeq:      0,
		MessageSeqMax:   0,
		Facility:        "daemon",
		Severity:        "info",
		Timestamp:       time.Now(),
		ProcessID:       1,
		Hostname:        "localhost",
		ApplicationName: "testing",
		LogText:         []byte{},
		PaddingLen:      6,
	}

	queue := New([]string{global.NSTest}, global.DefaultMinQueueSize, &mockDeadline)

	// Warm-up to stabilize caches, allocator, CPU frequency, ect
	for i := 0; i < 1000; i++ {
		payloadTemplate.LogText = append(payloadTemplate.LogText, []byte(strconv.Itoa(i))...)
		queue.push(mockCtx, "key"+strconv.Itoa(i), payloadTemplate, time.Now())
		key, ok := queue.PopKey(context.Background())
		if !ok {
			b.Fatalf("expected no error while warming, but failed to pop key at iteration %d", i)
		}
		_, notExist := queue.DrainBucket(mockCtx, key)
		if notExist {
			b.Fatalf("expected no error while warming, but bucket drain failed at iteration %d", i)
		}
	}

	msgSizes := []int{20, 512, 2048, 9000} // Testing small, medium, larger-than-normal-mtu, and jumbo
	results := make(map[int]float64)

	for _, msgSize := range msgSizes {
		b.Run(fmt.Sprintf("MsgSize=%d", msgSize), func(b *testing.B) {
			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				payloadTemplate.LogText = bytes.Repeat([]byte("0"), msgSize)
				queue.push(mockCtx, "key"+strconv.Itoa(i), payloadTemplate, time.Now())
				key, ok := queue.PopKey(context.Background())
				if !ok {
					b.Fatalf("expected no error during benchmark, but failed to pop key at iteration %d", i)
				}
				_, notExist := queue.DrainBucket(mockCtx, key)
				if notExist {
					b.Fatalf("expected no error during benchmark, but bucket drain failed at iteration %d", i)
				}
			}

			b.StopTimer()
			nsPerOp := float64(time.Since(start).Nanoseconds()) / float64(b.N)
			results[msgSize] = nsPerOp
		})
	}

	const (
		maxLinearFactor    = 0.5 // must be <1 to reject linear
		maxJumpFactor      = 3.0 // adjacent sizes
		maxSlopeMultiplier = 4.0
	)

	// Sub-linear growth
	for i := 0; i < len(msgSizes); i++ {
		for j := i + 1; j < len(msgSizes); j++ {
			n1, n2 := msgSizes[i], msgSizes[j]
			t1, t2 := results[n1], results[n2]

			sizeRatio := float64(n2) / float64(n1)
			timeRatio := t2 / t1

			if timeRatio > sizeRatio*maxLinearFactor {
				b.Fatalf(
					"scaling violation: payload %d->%d grew %.2fx (linear %.2fx)",
					n1, n2, timeRatio, sizeRatio,
				)
			}
		}
	}

	// No cliffs
	for i := 0; i < len(msgSizes)-1; i++ {
		n1, n2 := msgSizes[i], msgSizes[i+1]
		t1, t2 := results[n1], results[n2]

		if t2/t1 > maxJumpFactor {
			b.Fatalf(
				"unexpected jump: %d->%d bytes grew %.2fx",
				n1, n2, t2/t1,
			)
		}
	}

	// Slope stability
	prevSlope := -1.0
	for i := 0; i < len(msgSizes)-1; i++ {
		n1, n2 := msgSizes[i], msgSizes[i+1]
		t1, t2 := results[n1], results[n2]

		slope := (t2 - t1) / float64(n2-n1)

		if prevSlope > 0 && slope/prevSlope > maxSlopeMultiplier {
			b.Fatalf(
				"slope explosion: dT/dN jumped %.2fx",
				slope/prevSlope,
			)
		}
		prevSlope = slope
	}
}
