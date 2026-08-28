package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSecretPair_PlaintextInput(t *testing.T) {
	plaintext, enc, err := resolveSecretPair("secrets/db/password.yaml", "secrets/")
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "secrets/db/password.yaml" {
		t.Errorf("plaintext = %q, want %q", plaintext, "secrets/db/password.yaml")
	}
	if enc != "secrets/db/password.yaml.enc" {
		t.Errorf("enc = %q, want %q", enc, "secrets/db/password.yaml.enc")
	}
}

func TestResolveSecretPair_EncInput(t *testing.T) {
	plaintext, enc, err := resolveSecretPair("secrets/db/password.yaml.enc", "secrets/")
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "secrets/db/password.yaml" {
		t.Errorf("plaintext = %q, want %q", plaintext, "secrets/db/password.yaml")
	}
	if enc != "secrets/db/password.yaml.enc" {
		t.Errorf("enc = %q, want %q", enc, "secrets/db/password.yaml.enc")
	}
}

func TestResolveSecretPair_OutsideSecretsDir(t *testing.T) {
	_, _, err := resolveSecretPair("../etc/passwd.yaml", "secrets/")
	if err == nil {
		t.Fatal("expected error for path outside secrets dir")
	}
}

func TestResolveSecretPair_NestedPath(t *testing.T) {
	plaintext, enc, err := resolveSecretPair("secrets/apps/api-token.yaml", "secrets/")
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "secrets/apps/api-token.yaml" {
		t.Errorf("plaintext = %q, want %q", plaintext, "secrets/apps/api-token.yaml")
	}
	if enc != "secrets/apps/api-token.yaml.enc" {
		t.Errorf("enc = %q, want %q", enc, "secrets/apps/api-token.yaml.enc")
	}
}

func TestResolveSecretPair_NestedEncPath(t *testing.T) {
	plaintext, enc, err := resolveSecretPair("secrets/apps/api-token.yaml.enc", "secrets/")
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "secrets/apps/api-token.yaml" {
		t.Errorf("plaintext = %q, want %q", plaintext, "secrets/apps/api-token.yaml")
	}
	if enc != "secrets/apps/api-token.yaml.enc" {
		t.Errorf("enc = %q, want %q", enc, "secrets/apps/api-token.yaml.enc")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(path) {
		t.Error("expected fileExists to return true for existing file")
	}
	if fileExists(filepath.Join(dir, "nope.txt")) {
		t.Error("expected fileExists to return false for missing file")
	}
}

func TestDiscoverSecrets(t *testing.T) {
	dir := t.TempDir()
	// Create secrets
	if err := os.MkdirAll(filepath.Join(dir, "apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "db.yaml"), []byte("v: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "db.yaml.enc"), []byte("encrypted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "apps", "api.yaml"), []byte("v: 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := discoverSecrets(dir + "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(entries), entries)
	}
	if entries[0] != "apps/api" {
		t.Errorf("entries[0] = %q, want %q", entries[0], "apps/api")
	}
	if entries[1] != "db" {
		t.Errorf("entries[1] = %q, want %q", entries[1], "db")
	}
}

func TestDiscoverSecrets_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := discoverSecrets(filepath.Join(dir, "secrets") + "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestDiscoverSecrets_OnlyEnc(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secrets", "solo.yaml.enc"), []byte("encrypted"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := discoverSecrets(filepath.Join(dir, "secrets") + "/")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(entries), entries)
	}
	if entries[0] != "solo" {
		t.Errorf("entries[0] = %q, want %q", entries[0], "solo")
	}
}
