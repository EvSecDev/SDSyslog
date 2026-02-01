package logctx

import (
	"context"
	"runtime"
	"sdsyslog/internal/global"
	"testing"
)

func BenchmarkLogEvent_SingleProducer(b *testing.B) {
	ctx := context.Background()
	done := make(chan struct{})

	ctx = New(ctx, global.NSTest, 5, done)
	logger := GetLogger(ctx)

	if logger == nil {
		b.Fatal("logger is nil")
	}

	b.ReportAllocs()
	b.SetBytes(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "benchmark message %d", i)
	}
}

func BenchmarkLogEvent_MultiProducer(b *testing.B) {
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)

	ctx := context.Background()
	done := make(chan struct{})

	ctx = New(ctx, global.NSTest, 5, done)
	logger := GetLogger(ctx)

	if logger == nil {
		b.Fatal("logger is nil")
	}

	b.ReportAllocs()
	b.SetBytes(1)
	b.SetParallelism(8)

	const maxQueue = 10_000

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		localCount := 0
		for pb.Next() {
			LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "parallel benchmark message")
			localCount++

			if localCount%256 == 0 {
				logger.mutex.Lock()
				if len(logger.queue) > maxQueue {
					// Drop oldest events
					logger.queue = logger.queue[len(logger.queue)-maxQueue:]
				}
				logger.mutex.Unlock()
			}
		}
	})
}
