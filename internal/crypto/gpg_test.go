package crypto

import (
	"path/filepath"
	"testing"
)

func TestGPGRoundtrip(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.asc")

	provider := NewGPGProvider()
	if err := provider.Setup(keyPath, ""); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	plaintext := []byte("hello, world! this is a GPG secret.")

	ciphertext, err := provider.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := provider.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestGPGKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.asc")

	provider1 := NewGPGProvider()
	if err := provider1.Setup(keyPath, ""); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	provider2 := NewGPGProvider()
	if err := provider2.Setup(keyPath, ""); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	plaintext := []byte("persistence test")

	ciphertext, err := provider1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := provider2.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted = %q, want %q", decrypted, plaintext)
	}
}
