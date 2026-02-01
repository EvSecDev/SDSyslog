package hash

import (
	"crypto/sha512"
	"fmt"
)

// Creates hash of multiple byte slices
// Slices are combined in their input order
func MultipleSlices(inputs ...[]byte) (hash []byte, err error) {
	hasher := sha512.New()

	for _, input := range inputs {
		_, err = hasher.Write(input)
		if err != nil {
			err = fmt.Errorf("error writing data to hash: %w", err)
			return
		}
	}

	hash = hasher.Sum(nil)
	return
}
