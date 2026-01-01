package aead

import (
	"bytes"
	"crypto/rand"
	"sdsyslog/internal/crypto/ecdh"
	"testing"

	"golang.org/x/crypto/curve25519"
)

func TestEncryptDecrypt(t *testing.T) {
	key := []byte("example key 1234567890example123")
	nonce := []byte("nonce1234567")
	plaintext := []byte("This is a test message")
	associatedData := []byte("associated data")

	// Copy crypto values
	decrKey := make([]byte, len(key))
	copy(decrKey, key)

	ciphertext, err := Encrypt(plaintext, key, nonce, associatedData)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, decrKey, nonce, associatedData)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match the original. Got: %s, Want: %s", decrypted, plaintext)
	}
}

func TestDecryptWithAssociatedData(t *testing.T) {
	key := []byte("example key 1234567890example123")
	nonce := []byte("nonce1234567")
	plaintext := []byte("This is a test message")

	// Common use case
	suiteID := uint8(1)
	ephemPub := make([]byte, 32)
	rand.Read(ephemPub)

	// Exact use case
	ephemeralPriv := make([]byte, ecdh.KeyLen)
	rand.Read(ephemeralPriv)
	realPublic, _ := curve25519.X25519(ephemeralPriv, curve25519.Basepoint)

	// Define test cases with different encryption and decryption AAD variations
	tests := []struct {
		name          string
		encryptionAAD []byte // AAD for encryption
		decryptionAAD []byte // AAD for decryption
		expectedError bool   // Whether we expect an error
	}{
		// Real World data
		{"Runtime append with rand references", append(ephemPub, suiteID), append(ephemPub, suiteID), false},
		{"Runtime append with ecdh references", append(realPublic, suiteID), append(realPublic, suiteID), false},
		{"Runtime append with ecdh reverse references", append([]byte{suiteID}, realPublic...), append([]byte{suiteID}, realPublic...), false},

		// Basic cases
		{"Empty AAD for both", []byte{}, []byte{}, false},
		{"Same AAD for both", []byte("associated data"), []byte("associated data"), false},
		{"Different AADs", []byte("associated data"), []byte("wrong associated data"), true},

		// Odd and Even length cases
		{"Odd-length AAD for both", []byte{0x01}, []byte{0x01}, false},
		{"Even-length AAD for both", []byte{0x01, 0x02}, []byte{0x01, 0x02}, false},

		// Larger AAD cases
		{"Large AAD (32 bytes)", make([]byte, 32), make([]byte, 32), false},
		{"Large AAD (1024 bytes)", make([]byte, 1024), make([]byte, 1024), false},

		// Appending and Concatenating byte arrays
		{"Concatenated AAD", append([]byte("first part"), []byte("second part")...), append([]byte("first part"), []byte("second part")...), false},
		{"Appended AAD", append([]byte("associated data"), 0x01), append([]byte("associated data"), 0x01), false},

		// Padding and conversion cases
		{"Padded AAD", append([]byte("associated data"), make([]byte, 16)...), append([]byte("associated data"), make([]byte, 16)...), false},
		{"Base64 Encoded AAD", []byte("encodedAAD"), []byte("encodedAAD"), false},

		// Length mismatched cases
		{"Length-mismatched AAD", append([]byte("associated data"), 0x00), append([]byte("associated data"), 0x01), true},
	}

	// Copy crypto values for decryption
	decrKey := make([]byte, len(key))
	copy(decrKey, key)

	// Loop over the test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt with the provided AAD for encryption
			ciphertext, err := Encrypt(plaintext, key, nonce, tt.encryptionAAD)
			if err != nil {
				t.Fatalf("Encryption failed: %v", err)
			}

			// Decrypt with the provided AAD for decryption
			_, err = Decrypt(ciphertext, decrKey, nonce, tt.decryptionAAD)

			// Check the error condition
			if tt.expectedError && err == nil {
				t.Fatalf("Expected error when decrypting with mismatched associated data, but got none")
			} else if !tt.expectedError && err != nil {
				t.Fatalf("Expected success but decryption failed with error: %v", err)
			}
		})
	}
}

func TestDecryptWithTamperedCiphertext(t *testing.T) {
	key := []byte("example key 1234567890example123")
	nonce := []byte("nonce1234567")
	plaintext := []byte("This is a test message")
	associatedData := []byte("associated data")

	// Copy crypto values
	decrKey := make([]byte, len(key))
	copy(decrKey, key)

	// Encrypt the plaintext
	ciphertext, err := Encrypt(plaintext, key, nonce, associatedData)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Tamper with the ciphertext
	tamperedCiphertext := append(ciphertext[:len(ciphertext)-1], byte(ciphertext[len(ciphertext)-1]^0x01)) // Flip last byte

	// Attempt to decrypt the tampered ciphertext
	_, err = Decrypt(tamperedCiphertext, decrKey, nonce, associatedData)
	if err == nil {
		t.Fatal("Expected error when decrypting tampered ciphertext, but got none")
	}
}

func TestEncryptWithEmptyPlaintext(t *testing.T) {
	key := []byte("example key 1234567890example123")
	nonce := []byte("nonce1234567")
	emptyPlaintext := []byte("")
	associatedData := []byte("associated data")

	// Copy crypto values
	decrKey := make([]byte, len(key))
	copy(decrKey, key)

	// Encrypt the empty plaintext
	ciphertext, err := Encrypt(emptyPlaintext, key, nonce, associatedData)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Decrypt the ciphertext
	decrypted, err := Decrypt(ciphertext, decrKey, nonce, associatedData)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("Decrypted text should be empty, but got: %s", decrypted)
	}
}

func TestDecryptWithInvalidCiphertextLength(t *testing.T) {
	key := []byte("example key 1234567890example123")
	nonce := []byte("nonce1234567")
	incorrectCiphertext := []byte("invalid ciphertext") // Incorrect length or tampered data
	associatedData := []byte("associated data")

	// Attempt to decrypt with invalid ciphertext length
	_, err := Decrypt(incorrectCiphertext, key, nonce, associatedData)
	if err == nil {
		t.Fatal("Expected error when decrypting with invalid ciphertext, but got none")
	}
}

func TestEncryptWithInvalidKeyLength(t *testing.T) {
	invalidKey := []byte("short key") // Invalid key length for ChaCha20-Poly1305
	nonce := []byte("nonce1234567")
	plaintext := []byte("This is a test message")
	associatedData := []byte("associated data")

	// Attempt to encrypt with invalid key
	_, err := Encrypt(plaintext, invalidKey, nonce, associatedData)
	if err == nil {
		t.Fatal("Expected error when encrypting with invalid key length, but got none")
	}
}

func TestEncryptWithInvalidNonceLength(t *testing.T) {
	invalidKey := []byte("short key") // Invalid key length for ChaCha20-Poly1305
	nonce := []byte("nonce12345678")
	plaintext := []byte("This is a test message")
	associatedData := []byte("associated data")

	// Attempt to encrypt with invalid key
	_, err := Encrypt(plaintext, invalidKey, nonce, associatedData)
	if err == nil {
		t.Fatal("Expected error when encrypting with invalid key length, but got none")
	}
}

func TestEncryptDecryptLargeInput(t *testing.T) {
	key := []byte("example key 1234567890example123")
	nonce := []byte("nonce1234567")
	largePlaintext := make([]byte, 10*1024*1024) // 10 MB of data
	associatedData := []byte("associated data")

	// Copy crypto values
	decrKey := make([]byte, len(key))
	copy(decrKey, key)

	// Encrypt the large plaintext
	ciphertext, err := Encrypt(largePlaintext, key, nonce, associatedData)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Decrypt the ciphertext
	decrypted, err := Decrypt(ciphertext, decrKey, nonce, associatedData)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if !bytes.Equal(decrypted, largePlaintext) {
		t.Errorf("Decrypted data doesn't match the original large plaintext")
	}
}
