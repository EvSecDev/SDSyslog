package mpmc

import (
	"context"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"

	"github.com/pbnjay/memory"
)

// Resizes queue if nearing capacity limit or heavily unused
func (container *Queue[T]) ScaleCapacity(ctx context.Context) {
	activeQueue := container.ActiveWrite.Load()
	currentCapacity := activeQueue.Size
	currentDepth := activeQueue.Metrics.Depth.Load()

	// At minimum limit, nothing to do
	if currentCapacity <= container.minimumSize {
		return
	}
	// At maximum limit, nothing to do
	if currentCapacity >= container.maximumSize {
		return
	}

	// Check memory usage
	availMem := memory.FreeMemory()
	currentByteSize := activeQueue.Metrics.Bytes.Load()
	currentSizePerItem := currentByteSize / uint64(currentCapacity)

	// Estimate new queue maximum memory size in bytes
	expectedMaxNewQueueMemSize := uint64((nextPowerOfTwo(currentCapacity)) * int(currentSizePerItem))

	utilization := float64(currentDepth) / float64(currentCapacity) * 100
	// Decide direction
	var scaleUp, scaleDown bool
	if utilization >= 90 {
		// No scaling up when near system memory limit
		if availMem > 0 && expectedMaxNewQueueMemSize > availMem {
			return
		}

		scaleUp = true
	} else if utilization <= 2 {
		scaleDown = true
	}

	if scaleUp {
		newSize := uint64(nextPowerOfTwo(currentCapacity + 1))

		err := container.mutateSize(newSize)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"Failed to scale queue capacity: %v\n", err)
			return
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog,
			"Scaled up queue from %d to %d capacity\n", currentCapacity, nextPowerOfTwo(currentCapacity))
	} else if scaleDown {
		newSize := uint64(prevPowerOfTwo(currentCapacity))

		err := container.mutateSize(newSize)
		if err != nil {
			logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog,
				"Failed to scale queue capacity: %v\n", err)
			return
		}
		logctx.LogEvent(ctx, global.VerbosityProgress, global.InfoLog,
			"Scaled down queue from %d to %d capacity\n", currentCapacity, prevPowerOfTwo(currentCapacity))
	}
}

func nextPowerOfTwo(start int) (next int) {
	if start <= 1 {
		next = 1
		return
	}
	start--
	start |= start >> 1
	start |= start >> 2
	start |= start >> 4
	start |= start >> 8
	start |= start >> 16
	start |= start >> 32
	next = start + 1
	return
}

func prevPowerOfTwo(start int) (prev int) {
	if start == 0 {
		return
	}
	prev = nextPowerOfTwo(start) >> 1
	return
}

// Decides whether to scale up or down based on depth metric values (metric=depth)
func Trend(depthValues []uint64, queueSize int) (scaleUp bool, scaleDown bool) {
	n := len(depthValues)
	if n < 3 {
		return
	}

	const upThresholdPct = 70.0   // high watermark
	const downThresholdPct = 15.0 // low watermark
	const requireConsistent = 3   // trend must be consistently up/down

	// Compute occupancy percent of last value
	latestPct := float64(depthValues[n-1]) / float64(queueSize) * 100

	// Compute trend direction for each adjacent pair:
	// +1 = growing, -1 = shrinking, 0 = flat
	trend := 0
	consistentTrendCount := 1

	for i := n - 2; i >= 0 && consistentTrendCount < requireConsistent; i-- {
		diff := int64(depthValues[i+1]) - int64(depthValues[i])

		var d int
		switch {
		case diff > 0:
			d = 1
		case diff < 0:
			d = -1
		default:
			d = 0
		}

		if trend == 0 {
			trend = d
			continue
		}

		if d == trend {
			consistentTrendCount++
		} else {
			break
		}
	}

	// Scale UP
	if latestPct > upThresholdPct && trend > 0 && consistentTrendCount >= requireConsistent {
		scaleUp = true
		return
	}

	// Scale DOWN
	if latestPct < downThresholdPct && trend < 0 && consistentTrendCount >= requireConsistent {
		scaleDown = true
		return
	}

	return
}
