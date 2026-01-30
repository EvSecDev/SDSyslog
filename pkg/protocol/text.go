package protocol

// Converts a string to bytes, truncates to maxLength and removes non-printable ASCII characters.
// Handles empty values as per specification.
func cleanStringToBytes(input string, maxLength int) (cleanBytes []byte) {
	if input == "" {
		input = emptyFieldChar // mandatory empty placeholder
	}

	// Remove non-ASCII characters
	cleanBytes = make([]byte, 0, len(input))
	for _, r := range input {
		if r >= 0x20 && r <= 0x7E {
			cleanBytes = append(cleanBytes, byte(r))
		}
	}

	// Truncate to maxLength
	if len(cleanBytes) > maxLength {
		cleanBytes = cleanBytes[:maxLength]
	}
	return
}

// Removes null-byte sequences from data
func cleanBytes(data []byte) (cleanBytes []byte) {
	cleanBytes = data[:0]
	for _, b := range data {
		if b != 0 {
			cleanBytes = append(cleanBytes, b)
		}
	}
	return
}

// Checks if all bytes in slice are printable ASCII
func isPrintableASCII(data []byte) (ascii bool) {
	for _, b := range data {
		if b < 0x20 || b > 0x7E {
			return
		}
	}
	ascii = true
	return
}
