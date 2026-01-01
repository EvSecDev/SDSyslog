package crypto

// Overwrite slices' array with zeros
func Memzero(slice []byte) {
	for index := range slice {
		slice[index] = 0
	}
}
