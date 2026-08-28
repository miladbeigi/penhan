//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesProjectStructure(t *testing.T) {
	dir := newProject(t)

	stdout, stderr, code := runPenhan(t, dir, "init",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code != 0 {
		t.Fatalf("init failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	for _, want := range []string{"penhan.yaml", ".penhan/keys/aes.key", ".penhan/state.json", ".penhan/vault-token"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}

func TestAddThenEncryptSecret(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)

	secretContent := "super-secret-value"
	addSecret(t, dir, "db", secretContent)

	plainPath := filepath.Join(dir, "secrets", "db.yaml")
	plain, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatalf("add did not create secret file: %v", err)
	}
	if !strings.Contains(string(plain), secretContent) {
		t.Fatalf("secret file missing value: %s", plain)
	}

	stdout, stderr, code := runPenhan(t, dir, "encrypt", "secrets/db.yaml")
	if code != 0 {
		t.Fatalf("encrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	encPath := filepath.Join(dir, "secrets", "db.yaml.enc")
	data, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatalf("encrypted file not created: %v", err)
	}
	if strings.Contains(string(data), secretContent) {
		t.Error("encrypted file still contains plaintext")
	}
	if _, err := os.Stat(plainPath); !os.IsNotExist(err) {
		t.Errorf("encrypt must remove the plaintext file, got err=%v", err)
	}

	// The key saved by init must decrypt what encrypt produced, in a fresh process.
	stdout, stderr, code = runPenhan(t, dir, "decrypt", "secrets/db.yaml.enc")
	if code != 0 {
		t.Fatalf("decrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	decrypted, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatalf("decrypted file not created: %v", err)
	}
	if !strings.Contains(string(decrypted), secretContent) {
		t.Errorf("decrypted content mismatch: %s", decrypted)
	}
}

func TestAddListShowsSecret(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "api", "abc123")

	stdout, stderr, code := runPenhan(t, dir, "list")
	if code != 0 {
		t.Fatalf("list failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "api.yaml") {
		t.Errorf("list output missing api.yaml: %s", stdout)
	}
}
