// Basic calculation functions
package calc

import (
	"slices"
)

// Calculates mean of supplied values after removing percentage of extreme values (post-sort)
func TrimmedMeanUint64(values []uint64, trimPercent float64) (mean uint64) {
	if trimPercent < 0 {
		trimPercent = 0
	}

	numValues := len(values)
	if numValues == 0 {
		return
	}

	nums := make([]uint64, numValues)
	copy(nums, values)

	slices.Sort(nums)

	// How many values to drop from each end
	trimCount := int(float64(numValues) * trimPercent)
	if trimCount*2 >= numValues {
		trimCount = (numValues - 1) / 2
	}

	start := trimCount
	end := numValues - trimCount

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

	numValues := len(values)
	if numValues == 0 {
		return
	}

	// Copy and sort
	nums := make([]float64, numValues)
	copy(nums, values)
	slices.Sort(nums)

	// How many to trim from each end
	trimCount := int(float64(numValues) * trimPercent)
	if trimCount*2 >= numValues {
		trimCount = (numValues - 1) / 2
	}

	start := trimCount
	end := numValues - trimCount

	var sum float64
	count := float64(end - start)

	for _, v := range nums[start:end] {
		sum += v
	}

	mean = sum / count
	return
}
