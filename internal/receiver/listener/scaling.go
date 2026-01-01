package listener

// Decides whether to scale up or down based on how much time the worker spent doing anything (metric=busy_time_percent)
func Trend(busyTimes []float64) (scaleUp bool, scaleDown bool) {
	// Heuristics:
	// slope > 0.5 = trending upward
	// slope < -0.5 = trending downward
	const minTrendSlope = 0.5
	const scaleUpThresholdPct = 50.0
	const scaleDownThresholdPct = 20.0

	// Compute average for overall utilization context
	var sum float64
	for _, v := range busyTimes {
		sum += v
	}
	avg := sum / float64(len(busyTimes))

	// Trend detection via linear regression
	// x = 0..n-1, y = values[i]
	var (
		n   = float64(len(busyTimes))
		sx  float64
		sy  float64
		sxy float64
		sxx float64
	)
	for i, v := range busyTimes {
		x := float64(i)
		sx += x
		sy += v
		sxy += x * v
		sxx += x * x
	}

	// slope of the best-fit line
	denom := n*sxx - sx*sx
	if denom == 0 {
		return // pathological case, can't evaluate trend
	}
	slope := (n*sxy - sx*sy) / denom

	// Scale-UP conditions:
	// - average above threshold
	// - trending UP
	if avg > scaleUpThresholdPct && slope > minTrendSlope {
		scaleUp = true
		return
	}

	// Scale-DOWN conditions:
	// - average below threshold
	// - trending DOWN
	if avg < scaleDownThresholdPct && slope < -minTrendSlope {
		scaleDown = true
		return
	}
	return
}
