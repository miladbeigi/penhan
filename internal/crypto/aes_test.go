package crypto

import (
	"path/filepath"
	"testing"
)

func TestAESRoundtrip(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "aes.key")

	provider := NewAESProvider()
	if err := provider.Setup(keyPath, ""); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	plaintext := []byte("hello, world! this is a secret.")

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

func TestAESDifferentPlaintexts(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "aes.key")

	provider := NewAESProvider()
	if err := provider.Setup(keyPath, ""); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{name: "short", plaintext: "secret"},
		{name: "long", plaintext: "this is a much longer secret value with many characters"},
		{name: "special chars", plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{name: "unicode", plaintext: "こんにちは世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := provider.Encrypt([]byte(tt.plaintext))
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			decrypted, err := provider.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if string(decrypted) != tt.plaintext {
				t.Errorf("Decrypted = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestAESWithPassphrase(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "aes.key")

	provider := NewAESProvider()
	if err := provider.Setup(keyPath, "my-secret-passphrase"); err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	plaintext := []byte("passphrase-derived key test")

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
