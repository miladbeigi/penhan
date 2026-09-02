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
