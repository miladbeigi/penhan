//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushUploadsSecretToVault(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "db.yaml", "password: s3cret\n")

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push")
	if code != 0 {
		t.Fatalf("push failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	data := readVaultData(t, "db")
	if data["password"] != "s3cret" {
		t.Errorf("vault secret mismatch: got %v", data)
	}
}

func TestPullDownloadsSecretFromVault(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "api.yaml", "token: pulltest\n")
	mustPush(t, dir)

	// Simulate a fresh clone: no local secret files, no state.
	if err := os.Remove(filepath.Join(dir, "secrets", "api.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".penhan", "state.json")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "pull")
	if code != 0 {
		t.Fatalf("pull failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	encPath := filepath.Join(dir, "secrets", "api.yaml.enc")
	if _, err := os.Stat(encPath); err != nil {
		t.Fatalf("encrypted file not restored: %v", err)
	}
}

func TestPushPullRoundTrip(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "round.yaml", "password: roundtrip-value-42\n")
	mustPush(t, dir)

	if out, err := vaultCmd(t, "kv", "put", mountPath+"/round", "password=remote-modified"); err != nil {
		t.Fatalf("vault put failed: %v: %s", err, out)
	}

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "pull")
	if code != 0 {
		t.Fatalf("pull failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	stdout, stderr, code = runPenhan(t, dir, "decrypt", "secrets/round.yaml.enc")
	if code != 0 {
		t.Fatalf("decrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	decPath := filepath.Join(dir, "secrets", "round.yaml")
	data, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatalf("read decrypted: %v", err)
	}
	if !strings.Contains(string(data), "remote-modified") {
		t.Errorf("expected remote-modified, got: %s", data)
	}
}

func TestListShowsPushedSecrets(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "a.yaml", "v: 1\n")
	writeSecret(t, dir, "b.yaml", "v: 2\n")
	mustPush(t, dir)

	stdout, stderr, code := runPenhan(t, dir, "list")
	if code != 0 {
		t.Fatalf("list failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "a.yaml") {
		t.Errorf("list missing a.yaml: %s", stdout)
	}
	if !strings.Contains(stdout, "b.yaml") {
		t.Errorf("list missing b.yaml: %s", stdout)
	}
}

func TestRemoveDeletesSecret(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "del.yaml", "v: gone\n")
	mustPush(t, dir)

	stdout, stderr, code := runPenhan(t, dir, "remove", "del", "--force")
	if code != 0 {
		t.Fatalf("remove failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	if _, err := os.Stat(filepath.Join(dir, "secrets", "del.yaml")); !os.IsNotExist(err) {
		t.Errorf("expected secret file removed, got err=%v", err)
	}

	if data, err := readVaultDataIfPresent(t, "del"); err == nil && len(data) > 0 {
		t.Errorf("expected vault secret deleted, got: %v", data)
	}
}

func TestPlanShowsPendingChanges(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "plan.yaml", "v: 1\n")

	stdout, stderr, code := runPenhan(t, dir, "plan")
	if code != 0 {
		t.Fatalf("plan failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "1 to add") {
		t.Errorf("plan should report the new local secret as pending add: %s", stdout)
	}
	if !strings.Contains(stdout, "plan (new)") {
		t.Errorf("plan should list the pending secret path: %s", stdout)
	}
}

func TestConflictDetectedWithoutForce(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "conf.yaml", "v: original\n")
	mustPush(t, dir)

	if out, err := vaultCmd(t, "kv", "put", mountPath+"/conf", "v=remote-change"); err != nil {
		t.Fatalf("vault put: %v: %s", err, out)
	}

	writeSecret(t, dir, "conf.yaml", "v: local-change\n")

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push")
	if code == 0 {
		t.Errorf("expected push to fail on conflict, got success: %s", stdout)
	}
	if !strings.Contains(strings.ToLower(stdout+stderr), "conflict") {
		t.Errorf("expected conflict message, got stdout=%s stderr=%s", stdout, stderr)
	}

	// The remote value must be untouched.
	data := readVaultData(t, "conf")
	if data["v"] != "remote-change" {
		t.Errorf("conflict push must not overwrite remote, got: %v", data)
	}
}

func TestForceOverridesConflict(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "force.yaml", "v: original\n")
	mustPush(t, dir)

	if out, err := vaultCmd(t, "kv", "put", mountPath+"/force", "v=remote-change"); err != nil {
		t.Fatalf("vault put: %v: %s", err, out)
	}

	writeSecret(t, dir, "force.yaml", "v: local-force\n")

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push", "--force")
	if code != 0 {
		t.Fatalf("force push failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	data := readVaultData(t, "force")
	if data["v"] != "local-force" {
		t.Errorf("expected local-force, got: %v", data["v"])
	}
}
