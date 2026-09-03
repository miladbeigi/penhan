//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	s := newVaultSafe(t)
	secretContent := "password: super-secret-value\n"
	writeSecret(t, s, "db.yaml", secretContent)

	stdout, stderr, code := runPenhan(t, s.Dir, "encrypt", "secrets/db.yaml")
	if code != 0 {
		t.Fatalf("encrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	enc, err := os.ReadFile(filepath.Join(s.Dir, "secrets", "db.yaml.enc"))
	if err != nil {
		t.Fatalf("encrypted file not created: %v", err)
	}
	if strings.Contains(string(enc), "super-secret-value") {
		t.Error("encrypted file still contains plaintext")
	}
	if fileExists(t, s, "secrets/db.yaml") {
		t.Error("encrypt must remove the plaintext file")
	}

	// The key saved by add must decrypt what encrypt produced, in a fresh process.
	stdout, stderr, code = runPenhan(t, s.Dir, "decrypt", "secrets/db.yaml.enc")
	if code != 0 {
		t.Fatalf("decrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	dec, err := os.ReadFile(filepath.Join(s.Dir, "secrets", "db.yaml"))
	if err != nil {
		t.Fatalf("decrypted file not created: %v", err)
	}
	if string(dec) != secretContent {
		t.Errorf("decrypt round-trip mismatch:\n got: %q\nwant: %q", dec, secretContent)
	}
}

func TestEncryptDecryptDefaultToSecretsDir(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "one.yaml", "v: 1\n")
	writeSecret(t, s, "sub/two.yaml", "v: 2\n")

	stdout, stderr, code := runPenhan(t, s.Dir, "encrypt")
	if code != 0 {
		t.Fatalf("no-arg encrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	for _, want := range []string{"secrets/one.yaml.enc", "secrets/sub/two.yaml.enc"} {
		if !fileExists(t, s, want) {
			t.Errorf("no-arg encrypt should produce %s", want)
		}
	}

	stdout, stderr, code = runPenhan(t, s.Dir, "decrypt")
	if code != 0 {
		t.Fatalf("no-arg decrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	for _, want := range []string{"secrets/one.yaml", "secrets/sub/two.yaml"} {
		if !fileExists(t, s, want) {
			t.Errorf("no-arg decrypt should restore %s", want)
		}
	}
}

func TestErrorsDoNotPrintUsage(t *testing.T) {
	s := newVaultSafe(t)

	stdout, stderr, code := runPenhan(t, s.Dir, "encrypt", "secrets/ghost.yaml")
	if code == 0 {
		t.Fatal("encrypting a missing file must fail")
	}
	if strings.Contains(stdout+stderr, "Usage:") {
		t.Errorf("runtime errors must not dump the usage block: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestRemovedCommandsAreGone(t *testing.T) {
	s := newVaultSafe(t)
	for _, cmd := range []string{"init", "list", "plan", "pull", "remove", "safe"} {
		_, _, code := runPenhan(t, s.Dir, cmd)
		if code == 0 {
			t.Errorf("command %q should no longer exist", cmd)
		}
	}
}
