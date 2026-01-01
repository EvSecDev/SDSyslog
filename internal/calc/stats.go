// Basic calculation functions
package calc

import "sort"

// Calculates mean of supplied values after removing percentage of extreme values (post-sort)
func TrimmedMeanUint64(values []uint64, trimPercent float64) (mean uint64) {
	if trimPercent < 0 {
		trimPercent = 0
	}

	n := len(values)
	if n == 0 {
		return
	}

	nums := make([]uint64, n)
	copy(nums, values)

	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })

	// How many values to drop from each end
	trimCount := int(float64(n) * trimPercent)
	if trimCount*2 >= n {
		trimCount = (n - 1) / 2
	}

	start := trimCount
	end := n - trimCount

	var sum uint64
	count := end - start

	for _, v := range nums[start:end] {
		sum += v
	}

	mean = sum / uint64(count)
	return
}

// Calculates mean of supplied values after removing percentage of extreme values (post-sort)
func TrimmedMeanFloat64(values []float64, trimPercent float64) (mean float64) {
	if trimPercent < 0 {
		trimPercent = 0
	}

	n := len(values)
	if n == 0 {
		return
	}

	// Copy and sort
	nums := make([]float64, n)
	copy(nums, values)
	sort.Float64s(nums)

	// How many to trim from each end
	trimCount := int(float64(n) * trimPercent)
	if trimCount*2 >= n {
		trimCount = (n - 1) / 2
	}

	start := trimCount
	end := n - trimCount

	var sum float64
	count := float64(end - start)

	for _, v := range nums[start:end] {
		sum += v
	}

	mean = sum / count
	return
}
