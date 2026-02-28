package crypto

// Overwrite slices' array with zeros
func Memzero(slice []byte) {
	for index := range slice {
		slice[index] = 0
	}
}

// Checks if all bytes in slice are zero
func IsZero(slice []byte) (empty bool) {
	for _, b := range slice {
		if b != 0 {
			empty = false
			return
		}
	}
	empty = true
	return
}
