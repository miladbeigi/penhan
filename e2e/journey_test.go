//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initProjectWithTokenFile initializes a project using --vault-token-file,
// the recommended non-interactive flow.
func initProjectWithTokenFile(t *testing.T, dir string, v vaultEnv) {
	t.Helper()
	tokenFile := writeTokenFile(t, dir, v)
	stdout, stderr, code := run(t, dir, "", "init",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+v.addr,
		"--vault-token-file="+tokenFile,
	)
	requireSuccess(t, "init", stdout, stderr, code)
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func requireFile(t *testing.T, dir, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		t.Fatalf("expected %s to exist: %v", rel, err)
	}
}

func requireNoFile(t *testing.T, dir, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, got err=%v", rel, err)
	}
}

// TestFullJourneyEncryptDecrypt covers the core single-user journey:
// initialize, write a secret file, encrypt it, decrypt it back, and list it.
func TestFullJourneyEncryptDecrypt(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)

	initProjectWithTokenFile(t, dir, vault)

	for _, want := range []string{"penhan.yaml", ".penhan/keys/aes.key", ".penhan/state.json", ".penhan/vault-token"} {
		requireFile(t, dir, want)
	}

	content := "password: hunter2\n"
	writeFile(t, dir, "secrets/db.yaml", content)

	stdout, stderr, code := run(t, dir, "", "encrypt", "secrets/db.yaml")
	requireSuccess(t, "encrypt", stdout, stderr, code)
	requireFile(t, dir, "secrets/db.yaml.enc")
	requireNoFile(t, dir, "secrets/db.yaml")
	if encrypted := readFile(t, dir, "secrets/db.yaml.enc"); strings.Contains(encrypted, "hunter2") {
		t.Error("encrypted file still contains plaintext")
	}

	stdout, stderr, code = run(t, dir, "", "decrypt", "secrets/db.yaml.enc")
	requireSuccess(t, "decrypt", stdout, stderr, code)
	if got := readFile(t, dir, "secrets/db.yaml"); got != content {
		t.Errorf("decrypt round-trip mismatch:\n got: %q\nwant: %q", got, content)
	}

	stdout, stderr, code = run(t, dir, "", "list")
	requireSuccess(t, "list", stdout, stderr, code)
	if !strings.Contains(stdout, "db.yaml") {
		t.Errorf("list output missing db.yaml:\n%s", stdout)
	}
}
