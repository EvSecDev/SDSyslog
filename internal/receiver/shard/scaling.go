package shard

// Decides whether to scale up or down based on total buckets in shard (metric=total_buckets)
func Trend(bucketCounts []uint64) (scaleUp bool, scaleDown bool) {
	numBuckets := len(bucketCounts)
	if numBuckets < 2 {
		return
	}

	// Compute deltas
	deltas := make([]float64, numBuckets-1)
	for index := 1; index < numBuckets; index++ {
		delta := float64(bucketCounts[index]) - float64(bucketCounts[index-1])
		// Clamp delta to ignore spikes
		const maxDelta = 10.0
		if delta > maxDelta {
			delta = maxDelta
		} else if delta < -maxDelta {
			delta = -maxDelta
		}
		deltas[index-1] = delta
	}

	// Weighted smoothing (linear weights)
	var weightedSum float64
	var weightSum float64
	for i, delta := range deltas {
		weight := float64(i + 1) // older intervals get less weight
		weightedSum += delta * weight
		weightSum += weight
	}
	trend := weightedSum / weightSum

	// Thresholds for scaling
	const upThreshold = 5.0
	const downThreshold = 2.0

	if trend > upThreshold {
		scaleUp = true
	} else if trend < -downThreshold {
		scaleDown = true
	}

	return
}

// Decides whether to increase or decrease timeout value for buckets (account for high latency network links)
func TrendLatency(sumSpacing, totalFragments, timedOutFragments []uint64) (stepUp bool, stepDown bool) {
	numFragments := len(totalFragments)
	if numFragments == 0 {
		return
	}

	timeoutRatios := make([]float64, numFragments)
	avgSpacings := make([]float64, numFragments)

	for i := 0; i < numFragments; i++ {
		if totalFragments[i] == 0 {
			timeoutRatios[i] = 0
			avgSpacings[i] = 0
		} else {
			timeoutRatios[i] = float64(timedOutFragments[i]) / float64(totalFragments[i])
			avgSpacings[i] = float64(sumSpacing[i]) / float64(totalFragments[i])
		}
	}

	// Compute average timeout ratio and spacing trend
	var sumRatio float64
	for i := 0; i < numFragments; i++ {
		sumRatio += timeoutRatios[i]
	}
	avgTimeoutRatio := sumRatio / float64(numFragments)

	spacingSlope := (avgSpacings[numFragments-1] - avgSpacings[0]) / float64(numFragments)

	// Step up conditions
	if avgTimeoutRatio > 0.05 || spacingSlope > 0 {
		stepUp = true
	}

	// Step down conditions
	if avgTimeoutRatio < 0.01 && spacingSlope < 0 {
		stepDown = true
	}

	// Prevent simultaneous up and down
	if stepUp && stepDown {
		stepDown = false
	}

	return
}
