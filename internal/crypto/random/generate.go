package random

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
)

// Generates random integer that would fit into a uint32
func FourByte() (randInt int, err error) {
	var b [4]byte

	_, err = rand.Read(b[:])
	if err != nil {
		return
	}

	randInt = int(binary.BigEndian.Uint32(b[:]))
	return
}

// Fixes any insecure patterns found in slice input.
// Insecure can mean: empty, nil, all identical values.
// Modifies slice directly so all references are updated.
func PopulateEmptySlice(slice *[]byte, size int) (err error) {
	// Check if the slice is nil or empty (len on nil is always 0)
	if len(*slice) == 0 {
		// Allocate new based on requested size
		*slice = make([]byte, size)
	}

	// Check for insecure conditions
	if isAllIdentical(*slice) || isAllZero(*slice) {
		// Populate array with secure random values
		_, err = rand.Read(*slice)
		if err != nil {
			err = fmt.Errorf("failed to populate slice with pseudo random data: %w", err)
			return
		}
	}

	return
}

// Checks if all bytes in the array are the same
func isAllIdentical(slice []byte) bool {
	if len(slice) == 0 {
		// Should never occur when called from PopulateEmptySlice
		return true // trigger unhappy path
	}
	// Compare each byte with the first byte
	first := slice[0]
	for _, b := range slice[1:] {
		if b != first {
			return false
		}
	}
	return true
}

// Checks if the byte array is filled with zeroes
func isAllZero(slice []byte) bool {
	for _, b := range slice {
		if b != 0 {
			return false
		}
	}
	return true
}

// Generates random integer between two numbers (including the min/max)
func NumberInRange(min, max int) (randomNumber int, err error) {
	// Ensure min < max
	if min > max {
		err = fmt.Errorf("min must be less than or equal to max")
		return
	}

	// Generate a random number in [min, max]
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		err = fmt.Errorf("failed reading in range: %w", err)
		return
	}

	randomNumber = int(n.Int64()) + min
	return
}
