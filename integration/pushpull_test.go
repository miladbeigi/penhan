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
	if _, _, code := runPenhan(t, dir, "add", "db.yaml"); code != 0 {
		t.Fatalf("add failed")
	}

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
	runPenhan(t, dir, "add", "api.yaml")
	runPenhanWithStdin(t, dir, []byte("y\n"), "push")

	if err := os.Remove(filepath.Join(dir, "secrets", "api.yaml.enc")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".penhan", "state.json")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, dir, "pull")
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
	runPenhan(t, dir, "add", "round.yaml")
	runPenhanWithStdin(t, dir, []byte("y\n"), "push")

	if _, err := vaultCmd(t, "kv", "put", mountPath+"/round", "password=remote-modified"); err != nil {
		t.Fatalf("vault put failed: %v", err)
	}

	runPenhan(t, dir, "pull")

	stdout, stderr, code := runPenhan(t, dir, "decrypt", "secrets/round.yaml.enc")
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
	runPenhan(t, dir, "add", "a.yaml")
	runPenhan(t, dir, "add", "b.yaml")
	runPenhanWithStdin(t, dir, []byte("y\n"), "push")

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
	runPenhan(t, dir, "add", "del.yaml")
	runPenhanWithStdin(t, dir, []byte("y\n"), "push")

	stdout, stderr, code := runPenhan(t, dir, "remove", "del.yaml", "--force")
	if code != 0 {
		t.Fatalf("remove failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	if _, err := os.Stat(filepath.Join(dir, "secrets", "del.yaml.enc")); !os.IsNotExist(err) {
		t.Errorf("expected encrypted file removed, got err=%v", err)
	}

	if _, err := vaultCmd(t, "kv", "get", mountPath+"/del"); err == nil {
		t.Error("expected vault get to fail after remove")
	}
}

func TestPlanShowsPendingChanges(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "plan.yaml", "v: 1\n")
	runPenhan(t, dir, "add", "plan.yaml")

	stdout, stderr, code := runPenhan(t, dir, "plan")
	if code != 0 {
		t.Fatalf("plan failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(strings.ToLower(stdout), "add") {
		t.Errorf("plan output missing 'add': %s", stdout)
	}
}

func TestConflictDetectedWithoutForce(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "conf.yaml", "v: original\n")
	runPenhan(t, dir, "add", "conf.yaml")
	runPenhanWithStdin(t, dir, []byte("y\n"), "push")

	if _, err := vaultCmd(t, "kv", "put", mountPath+"/conf", "v=remote-change"); err != nil {
		t.Fatalf("vault put: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "secrets", "conf.yaml"), []byte("v: local-change\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push")
	if code == 0 {
		t.Error("expected push to fail on conflict, got success")
	}
}

func TestForceOverridesConflict(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "force.yaml", "v: original\n")
	runPenhan(t, dir, "add", "force.yaml")
	runPenhanWithStdin(t, dir, []byte("y\n"), "push")

	if _, err := vaultCmd(t, "kv", "put", mountPath+"/force", "v=remote-change"); err != nil {
		t.Fatalf("vault put: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "secrets", "force.yaml"), []byte("v: local-force\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push", "--force")
	if code != 0 {
		t.Fatalf("force push failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	data := readVaultData(t, "force")
	if data["v"] != "local-force" {
		t.Errorf("expected local-force, got: %v", data["v"])
	}
}
