//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// assertSameSecretData parses both sides as YAML/JSON and compares the data,
// not the raw bytes: pull canonicalizes content to JSON, which is valid YAML
// carrying identical key-value data.
func assertSameSecretData(t *testing.T, got, want string) {
	t.Helper()
	var gotData, wantData map[string]interface{}
	if err := yaml.Unmarshal([]byte(got), &gotData); err != nil {
		t.Fatalf("parse pulled content %q as YAML: %v", got, err)
	}
	if err := yaml.Unmarshal([]byte(want), &wantData); err != nil {
		t.Fatalf("parse original content %q as YAML: %v", want, err)
	}
	if len(gotData) != len(wantData) {
		t.Fatalf("secret data length mismatch: got %v, want %v", gotData, wantData)
	}
	for k, v := range wantData {
		if gotData[k] != v {
			t.Errorf("key %q: got %v, want %v", k, gotData[k], v)
		}
	}
}

// TestSyncJourneyPushPull covers the two-machine flow: project A pushes
// secrets to Vault, a freshly initialized project B pulls them back and
// decrypts them with its own key.
func TestSyncJourneyPushPull(t *testing.T) {
	vault := startVault(t)

	content := "password: hunter2\n"

	// --- Project A: create and push ---
	dirA := newProject(t)
	initProjectWithTokenFile(t, dirA, vault)

	writeFile(t, dirA, "secrets/db.yaml", content)

	stdout, stderr, code := run(t, dirA, "y\n", "push")
	requireSuccess(t, "push", stdout, stderr, code)
	if !strings.Contains(stdout, "Pushed: db") {
		t.Errorf("push output missing 'Pushed: db':\n%s", stdout)
	}
	requireFile(t, dirA, ".penhan/state.json")

	// --- Project B: pull from a fresh start ---
	dirB := newProject(t)
	initProjectWithTokenFile(t, dirB, vault)

	stdout, stderr, code = run(t, dirB, "y\n", "pull")
	requireSuccess(t, "pull", stdout, stderr, code)
	requireFile(t, dirB, "secrets/db.yaml.enc")

	stdout, stderr, code = run(t, dirB, "", "decrypt", "secrets/db.yaml.enc")
	requireSuccess(t, "decrypt", stdout, stderr, code)
	assertSameSecretData(t, readFile(t, dirB, "secrets/db.yaml"), content)
}

// TestPushIsIdempotent verifies that pushing an unchanged project twice
// succeeds and the second push reports no additions.
func TestPushIsIdempotent(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	initProjectWithTokenFile(t, dir, vault)

	writeFile(t, dir, "secrets/db.yaml", "password: hunter2\n")

	stdout, stderr, code := run(t, dir, "y\n", "push")
	requireSuccess(t, "first push", stdout, stderr, code)
	if !strings.Contains(stdout, "+ 1 to add") {
		t.Errorf("first push should plan 1 addition:\n%s", stdout)
	}

	stdout, stderr, code = run(t, dir, "y\n", "push")
	requireSuccess(t, "second push", stdout, stderr, code)
	if strings.Contains(stdout, "+ 1 to add") {
		t.Errorf("second push should not plan additions:\n%s", stdout)
	}
}
