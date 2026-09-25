package commands

import (
	"os"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/config"
)

func TestLoadSafeConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg, err := loadSafeConfig()
	if err == nil {
		t.Fatal("expected error when penhan.yaml is missing")
	}
	if cfg != nil {
		t.Fatal("expected nil config on error")
	}

	want := "no penhan.yaml in the current directory; run this command inside a safe (created with `penhan add`)"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

// Safes created before github-gpg was removed must get a migration hint,
// not a generic "unsupported" error.
func TestNewCryptoProvider_GitHubGPGRemoved(t *testing.T) {
	cfg := &config.Config{Encryption: config.EncryptionConfig{Method: "github-gpg"}}
	_, err := loadCryptoProvider(cfg)
	if err == nil {
		t.Fatal("expected an error for the removed github-gpg method")
	}
	if !strings.Contains(err.Error(), "no longer supported") {
		t.Errorf("error should explain github-gpg was removed, got %q", err)
	}
}
