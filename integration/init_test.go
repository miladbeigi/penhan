package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesProjectStructure(t *testing.T) {
	dir := newProject(t)

	stdout, stderr, code := runPenhanWithStdin(t, dir,
		[]byte("aes\nvault\n"+vaultAddr+"\n"+vaultToken+"\n"),
		"init")
	if code != 0 {
		t.Fatalf("init failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	for _, want := range []string{"penhan.yaml", ".penhan/keys/aes.key", ".penhan/state.json", ".penhan/vault-token"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}

func TestAddEncryptsSecret(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)

	secretContent := "super-secret-value"
	if err := os.WriteFile(filepath.Join(dir, "secrets", "db.yaml"), []byte("password: "+secretContent+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, dir, "add", "db.yaml")
	if code != 0 {
		t.Fatalf("add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	encPath := filepath.Join(dir, "secrets", "db.yaml.enc")
	data, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatalf("encrypted file not created: %v", err)
	}
	if strings.Contains(string(data), secretContent) {
		t.Error("encrypted file still contains plaintext")
	}
}

func TestAddListShowsSecret(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "api.yaml", "key: abc123\n")

	stdout, stderr, code := runPenhan(t, dir, "add", "api.yaml")
	if code != 0 {
		t.Fatalf("add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	stdout, stderr, code = runPenhan(t, dir, "list")
	if code != 0 {
		t.Fatalf("list failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "api.yaml") {
		t.Errorf("list output missing api.yaml: %s", stdout)
	}
}
