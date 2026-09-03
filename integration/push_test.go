//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushUploadsSecretUnderSafeBasePath(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "db.yaml", "password: s3cret\n")

	stdout := mustPush(t, s)
	if !strings.Contains(stdout, "Pushed (new): db") {
		t.Errorf("push should report the new secret:\n%s", stdout)
	}

	data := readVaultData(t, s, "db")
	if data["password"] != "s3cret" {
		t.Errorf("vault secret mismatch: got %v", data)
	}
}

func TestPushNestedSecret(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "apps/api-token.yaml", "token: nested-tok\n")
	mustPush(t, s)

	data := readVaultData(t, s, "apps/api-token")
	if data["token"] != "nested-tok" {
		t.Errorf("nested secret mismatch: got %v", data)
	}
	if out := mustCheck(t, s); !strings.Contains(out, "unchanged  apps/api-token") {
		t.Errorf("check should settle after pushing a nested secret:\n%s", out)
	}
}

func TestPushSkipsUnchangedSecrets(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "db.yaml", "password: hunter2\n")
	mustPush(t, s)

	stdout := mustPush(t, s)
	if !strings.Contains(stdout, "Unchanged: db") {
		t.Errorf("second push should report db as unchanged:\n%s", stdout)
	}
	if strings.Contains(stdout, "Pushed") {
		t.Errorf("second push must not push anything:\n%s", stdout)
	}
	if !strings.Contains(stdout, "0 pushed, 1 unchanged") {
		t.Errorf("second push summary wrong:\n%s", stdout)
	}
}

func TestPushOnlyChangedSecrets(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "a.yaml", "v: 1\n")
	writeSecret(t, s, "b.yaml", "v: 2\n")
	mustPush(t, s)

	writeSecret(t, s, "b.yaml", "v: 3\n")
	stdout := mustPush(t, s)
	if !strings.Contains(stdout, "Unchanged: a") || !strings.Contains(stdout, "Pushed (changed): b") {
		t.Errorf("push should skip a and push b:\n%s", stdout)
	}
	if data := readVaultData(t, s, "b"); data["v"] != "3" {
		t.Errorf("b should be updated in vault, got %v", data)
	}
}

func TestPushOverwritesRemoteEdits(t *testing.T) {
	// Without a state file there is no conflict detection: the local file is
	// the source of truth and push restores it when the remote drifted.
	s := newVaultSafe(t)
	writeSecret(t, s, "drift.yaml", "v: original\n")
	mustPush(t, s)

	vaultPut(t, s, "drift", "v=remote-edit")
	if out := mustCheck(t, s); !strings.Contains(out, "changed    drift") {
		t.Errorf("check should flag a remote edit as changed:\n%s", out)
	}

	stdout := mustPush(t, s)
	if !strings.Contains(stdout, "Pushed (changed): drift") {
		t.Errorf("push should re-push the drifted secret:\n%s", stdout)
	}
	if data := readVaultData(t, s, "drift"); data["v"] != "original" {
		t.Errorf("push should restore the local value, got %v", data)
	}
}

func TestPushReadsEncryptedOnlySecret(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "sealed.yaml", "k: sealed-value\n")
	if _, _, code := runPenhan(t, s.Dir, "encrypt"); code != 0 {
		t.Fatal("encrypt failed")
	}
	if fileExists(t, s, "secrets/sealed.yaml") {
		t.Fatal("precondition: plaintext must be gone after encrypt")
	}

	mustPush(t, s)
	if data := readVaultData(t, s, "sealed"); data["k"] != "sealed-value" {
		t.Errorf("push must read .enc files, got %v", data)
	}
}

func TestPushRejectsNestedValues(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "bad.yaml", "outer:\n  inner: 1\n")

	stdout, stderr, code := runPenhan(t, s.Dir, "push")
	if code == 0 {
		t.Fatalf("push must reject nested values: %s", stdout)
	}
	if !strings.Contains(stdout+stderr, "nested") {
		t.Errorf("error should explain the nested value: %s%s", stdout, stderr)
	}
}

func TestPushWithFileBackend(t *testing.T) {
	s := newFileSafe(t)
	writeSecret(t, s, "db.yaml", "password: on-disk\n")

	stdout := mustPush(t, s)
	if !strings.Contains(stdout, "Pushed (new): db") {
		t.Errorf("push should report the new secret:\n%s", stdout)
	}

	remote := filepath.Join(s.Dir, ".penhan", "remote", "db.enc")
	data, err := os.ReadFile(remote)
	if err != nil {
		t.Fatalf("file backend should write %s: %v", remote, err)
	}
	if strings.Contains(string(data), "on-disk") {
		t.Error("file backend copy must be encrypted")
	}

	if out := mustPush(t, s); !strings.Contains(out, "Unchanged: db") {
		t.Errorf("second push to file backend should be a no-op:\n%s", out)
	}
}
