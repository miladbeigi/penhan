package crypto

import (
	"path/filepath"
	"testing"
)

func TestGitHubGPGSealOnly(t *testing.T) {
	_, err := NewGitHubGPGProvider().Decrypt([]byte("data"))
	if err == nil {
		t.Fatal("expected seal-only error")
	}
	if err.Error() != "github-gpg provider is seal-only: decryption requires the private key on your local machine" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitHubGPGSetupNoUsername(t *testing.T) {
	err := NewGitHubGPGProvider().Setup("", "")
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestGitHubGPGNotInitialized(t *testing.T) {
	p := NewGitHubGPGProvider()
	if p.IsInitialized() {
		t.Fatal("should not be initialized")
	}
	if !p.SealOnly() {
		t.Fatal("github-gpg provider should be seal-only")
	}
}

func TestGitHubGPGSetupMissingKey(t *testing.T) {
	// Provide a non-existent cache path with no username — should fail.
	err := NewGitHubGPGProvider().Setup(filepath.Join(t.TempDir(), "nonexistent.pub"), "")
	if err == nil {
		t.Fatal("expected error")
	}
}
