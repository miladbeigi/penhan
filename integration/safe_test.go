//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/config"
)

func TestSafeAddCreatesProjectStructure(t *testing.T) {
	dir := newProject(t)
	safeDir := filepath.Join(dir, "default")

	stdout, stderr, code := runPenhan(t, dir, "safe", "add", "default",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code != 0 {
		t.Fatalf("safe add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	for _, want := range []string{"penhan.yaml", ".penhan/keys/aes.key", ".penhan/state.json", ".penhan/vault-token"} {
		if _, err := os.Stat(filepath.Join(safeDir, want)); err != nil {
			t.Errorf("expected %s to exist: %v", want, err)
		}
	}
}

func TestSafeAddSetsBasePath(t *testing.T) {
	dir := newProject(t)
	safeName := "mysafe"

	stdout, stderr, code := runPenhan(t, dir, "safe", "add", safeName,
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code != 0 {
		t.Fatalf("safe add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	cfg, err := config.Load(filepath.Join(dir, safeName, "penhan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend.Vault.BasePath != safeName {
		t.Errorf("base_path = %q, want %q", cfg.Backend.Vault.BasePath, safeName)
	}
}

func TestSafeAddRejectsDuplicate(t *testing.T) {
	dir := newProject(t)
	args := []string{"safe", "add", "dup",
		"--encryption=aes", "--backend=vault",
		"--vault-addr=" + vaultAddr, "--vault-token=" + vaultToken,
	}
	stdout, stderr, code := runPenhan(t, dir, args...)
	if code != 0 {
		t.Fatalf("first safe add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	stdout, stderr, code = runPenhan(t, dir, args...)
	if code == 0 {
		t.Fatal("second safe add must fail")
	}
	if !strings.Contains(stdout+stderr, "already exists") {
		t.Errorf("error should mention already exists: %s", stdout)
	}
}

func TestSafeListShowsSafes(t *testing.T) {
	dir := newProject(t)
	runPenhan(t, dir, "safe", "add", "alpha",
		"--encryption=aes", "--backend=vault",
		"--vault-addr="+vaultAddr, "--vault-token="+vaultToken,
	)
	runPenhan(t, dir, "safe", "add", "beta",
		"--encryption=aes", "--backend=vault",
		"--vault-addr="+vaultAddr, "--vault-token="+vaultToken,
	)

	stdout, stderr, code := runPenhan(t, dir, "safe", "list")
	if code != 0 {
		t.Fatalf("safe list failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "alpha") {
		t.Errorf("list should show alpha: %s", stdout)
	}
	if !strings.Contains(stdout, "beta") {
		t.Errorf("list should show beta: %s", stdout)
	}
}

func TestSafeListEmpty(t *testing.T) {
	dir := newProject(t)
	stdout, stderr, code := runPenhan(t, dir, "safe", "list")
	if code != 0 {
		t.Fatalf("safe list failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "No safes found") {
		t.Errorf("expected 'No safes found': %s", stdout)
	}
}
