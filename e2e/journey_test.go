//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestFullJourney covers the single-user flow end to end: create a safe,
// write a secret, encrypt it for git, check, push, and verify it landed in
// Vault under the safe's base path.
func TestFullJourney(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	safe := addSafe(t, dir, "myapp", vault)

	for _, want := range []string{"penhan.yaml", "secrets", ".penhan/keys/aes.key", ".penhan/vault-token"} {
		requireFile(t, safe, want)
	}
	requireNoFile(t, safe, ".penhan/state.json")
	requireFile(t, dir, ".gitignore")

	content := "password: hunter2\n"
	writeFile(t, safe, "secrets/db.yaml", content)

	stdout, stderr, code := run(t, safe, "", "encrypt", "secrets/db.yaml")
	requireSuccess(t, "encrypt", stdout, stderr, code)
	requireFile(t, safe, "secrets/db.yaml.enc")
	requireNoFile(t, safe, "secrets/db.yaml")
	if strings.Contains(readFile(t, safe, "secrets/db.yaml.enc"), "hunter2") {
		t.Error("encrypted file still contains plaintext")
	}

	// check and push both read the .enc copy, as they would in a fresh clone.
	stdout, stderr, code = run(t, safe, "", "check")
	requireSuccess(t, "check", stdout, stderr, code)
	requireContains(t, "check", stdout, "new        db")
	if vaultData(t, vault, "secret/data/myapp/db") != nil {
		t.Fatal("check must not write to Vault")
	}

	stdout, stderr, code = run(t, safe, "", "push")
	requireSuccess(t, "push", stdout, stderr, code)
	requireContains(t, "push", stdout, "Pushed (new): db")
	requireContains(t, "push", stdout, "Push complete: 1 pushed, 0 unchanged")

	data := vaultData(t, vault, "secret/data/myapp/db")
	if data == nil || data["password"] != "hunter2" {
		t.Fatalf("secret not in Vault under the safe base path, got %v", data)
	}

	stdout, stderr, code = run(t, safe, "", "decrypt", "secrets/db.yaml.enc")
	requireSuccess(t, "decrypt", stdout, stderr, code)
	if got := readFile(t, safe, "secrets/db.yaml"); got != content {
		t.Errorf("decrypt round-trip mismatch:\n got: %q\nwant: %q", got, content)
	}
}

// TestPushIsIdempotent verifies a second push skips everything and check
// agrees, while a local edit is picked up as changed.
func TestPushIsIdempotent(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	safe := addSafe(t, dir, "idem", vault)

	writeFile(t, safe, "secrets/db.yaml", "password: hunter2\n")

	stdout, stderr, code := run(t, safe, "", "push")
	requireSuccess(t, "first push", stdout, stderr, code)
	requireContains(t, "first push", stdout, "Pushed (new): db")

	stdout, stderr, code = run(t, safe, "", "push")
	requireSuccess(t, "second push", stdout, stderr, code)
	requireContains(t, "second push", stdout, "Unchanged: db")
	if strings.Contains(stdout, "Pushed") {
		t.Errorf("second push must not push anything:\n%s", stdout)
	}

	writeFile(t, safe, "secrets/db.yaml", "password: rotated\n")
	stdout, stderr, code = run(t, safe, "", "check")
	requireSuccess(t, "check", stdout, stderr, code)
	requireContains(t, "check", stdout, "changed    db")

	stdout, stderr, code = run(t, safe, "", "push")
	requireSuccess(t, "third push", stdout, stderr, code)
	requireContains(t, "third push", stdout, "Pushed (changed): db")
	if data := vaultData(t, vault, "secret/data/idem/db"); data["password"] != "rotated" {
		t.Errorf("Vault should hold the rotated value, got %v", data)
	}
}

// TestRemoteEditIsDetectedAndOverwritten documents the no-state model: an
// edit made directly in Vault shows up as changed, and push restores the
// local file's content.
func TestRemoteEditIsDetectedAndOverwritten(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	safe := addSafe(t, dir, "drift", vault)

	writeFile(t, safe, "secrets/db.yaml", "password: original\n")
	stdout, stderr, code := run(t, safe, "", "push")
	requireSuccess(t, "push", stdout, stderr, code)

	vaultWrite(t, vault, "secret/data/drift/db", map[string]interface{}{"password": "edited-in-vault"})

	stdout, stderr, code = run(t, safe, "", "check")
	requireSuccess(t, "check", stdout, stderr, code)
	requireContains(t, "check", stdout, "changed    db")

	stdout, stderr, code = run(t, safe, "", "push")
	requireSuccess(t, "push", stdout, stderr, code)
	requireContains(t, "push", stdout, "Pushed (changed): db")
	if data := vaultData(t, vault, "secret/data/drift/db"); data["password"] != "original" {
		t.Errorf("push should restore the local value, got %v", data)
	}
}
