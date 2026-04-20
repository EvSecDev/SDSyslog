package fsops

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// Creates hash of data from reader
func hashReader(reader io.Reader) (hash []byte, err error) {
	hasher := sha256.New()

	_, err = io.Copy(hasher, reader)
	if err != nil {
		err = fmt.Errorf("hash reader: %w", err)
		return
	}

	hash = hasher.Sum(nil)
	return
}

// Compares hashes of data from both readers
func readersEqual(readerA, readerB io.Reader) (isEqual bool, err error) {
	hashA, err := hashReader(readerA)
	if err != nil {
		return
	}

	hashB, err := hashReader(readerB)
	if err != nil {
		return
	}

	isEqual = bytes.Equal(hashA, hashB)
	return
}

// Compares hashes of a real file to data from reader
func FileEqualsReader(path string, reader io.Reader) (isEqual bool, err error) {
	file, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("open file: %w", err)
		return
	} else if err != nil && os.IsNotExist(err) {
		// No file - automatically not equal
		err = nil
		return
	}
	defer func() {
		_ = file.Close()
	}()

	isEqual, err = readersEqual(file, reader)
	return
}

// Compares the contents of two files
func FilesEqual(pathA, pathB string) (isEqual bool, err error) {
	fileA, err := os.Open(pathA)
	if err != nil && !os.IsNotExist(err) {
		return
	} else if err != nil && os.IsNotExist(err) {
		// No file - automatically not equal
		err = nil
		return
	}
	defer func() {
		_ = fileA.Close()
	}()

	fileB, err := os.Open(pathB)
	if err != nil && !os.IsNotExist(err) {
		return
	} else if err != nil && os.IsNotExist(err) {
		// No file - automatically not equal
		err = nil
		return
	}
	defer func() {
		_ = fileB.Close()
	}()

	isEqual, err = readersEqual(fileA, fileB)
	return
}
