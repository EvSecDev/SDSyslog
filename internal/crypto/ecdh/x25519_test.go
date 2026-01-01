package ecdh

import "testing"

func TestCreateSharedSecret(t *testing.T) {
	// Generate persistent key pair for sender (one side)
	senderPrivate, senderPublic, err := CreatePersistentKey()
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	// Generate shared secret using sender's public key
	sharedSecret, ephemeralPublic, err := CreateSharedSecret(senderPublic)
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	// Check that shared secret is the correct length
	if len(sharedSecret) != KeyLen {
		t.Errorf("Expected shared secret length of 32 bytes, but got %d bytes", len(sharedSecret))
	}

	// Check that the ephemeral public key is valid (32 bytes)
	if len(ephemeralPublic) != KeyLen {
		t.Errorf("Expected ephemeral public key length of 32 bytes, but got %d bytes", len(ephemeralPublic))
	}

	// Simulate receiver's side using the sender's private key and ephemeral public key
	receiverSharedSecret, err := ReCreateSharedSecret(senderPrivate, ephemeralPublic)
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	// Check that the shared secret computed by the receiver matches the sender's shared secret
	if string(sharedSecret) != string(receiverSharedSecret) {
		t.Errorf("Shared secret does not match between sender and receiver")
	}
}
