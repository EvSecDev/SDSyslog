package parsing

// Converts integer to unsigned, if negative, clamps to zero.
func ToUint64(integer int) (unsigned uint64) {
	if integer < 0 {
		return 0
	}
	unsigned = uint64(integer)
	return
}
