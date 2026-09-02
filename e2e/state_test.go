//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestPlanReflectsLocalChanges verifies plan reports additions before the
// first push and reports nothing after the secret is synced.
func TestPlanReflectsLocalChanges(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	initProjectWithTokenFile(t, dir, vault)

	stdout, stderr, code := run(t, dir, "", "plan")
	requireSuccess(t, "plan", stdout, stderr, code)
	if strings.Contains(stdout, "+ 1 to add") {
		t.Errorf("plan should be empty on a fresh project:\n%s", stdout)
	}

	stdout, stderr, code = run(t, dir, "", "add", "db")
	requireSuccess(t, "add", stdout, stderr, code)
	writeFile(t, dir, "secrets/db.yaml", "password: hunter2\n")

	stdout, stderr, code = run(t, dir, "", "plan")
	requireSuccess(t, "plan", stdout, stderr, code)
	if !strings.Contains(stdout, "+ 1 to add") {
		t.Errorf("plan should report the new secret:\n%s", stdout)
	}
	if !strings.Contains(stdout, "db (new)") {
		t.Errorf("plan should list db as new:\n%s", stdout)
	}

	stdout, stderr, code = run(t, dir, "y\n", "push")
	requireSuccess(t, "push", stdout, stderr, code)

	stdout, stderr, code = run(t, dir, "", "plan")
	requireSuccess(t, "plan", stdout, stderr, code)
	if strings.Contains(stdout, "+ 1 to add") {
		t.Errorf("plan should be empty after push:\n%s", stdout)
	}
}

// TestRemoveDeletesFromVault verifies that removing a pushed secret with
// --force drops the local files, the Vault entry, and leaves a clean plan.
func TestRemoveDeletesFromVault(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	initProjectWithTokenFile(t, dir, vault)

	stdout, stderr, code := run(t, dir, "", "add", "db")
	requireSuccess(t, "add", stdout, stderr, code)
	writeFile(t, dir, "secrets/db.yaml", "password: hunter2\n")

	stdout, stderr, code = run(t, dir, "y\n", "push")
	requireSuccess(t, "push", stdout, stderr, code)
	if vaultData(t, vault, "secret/data/db") == nil {
		t.Fatal("secret should exist in Vault after push")
	}

	stdout, stderr, code = run(t, dir, "", "remove", "secrets/db.yaml", "--force")
	requireSuccess(t, "remove", stdout, stderr, code)
	requireNoFile(t, dir, "secrets/db.yaml")

	if data := vaultData(t, vault, "secret/data/db"); data != nil {
		t.Errorf("secret should be deleted from Vault, got: %v", data)
	}

	stdout, stderr, code = run(t, dir, "", "plan")
	requireSuccess(t, "plan", stdout, stderr, code)
	if !strings.Contains(stdout, "- 0 to delete") {
		t.Errorf("plan should not report deletions after remove:\n%s", stdout)
	}
}
