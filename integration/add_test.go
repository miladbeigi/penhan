//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/config"
)

func TestAddCreatesSafeStructure(t *testing.T) {
	s := newVaultSafe(t)

	for _, want := range []string{"penhan.yaml", "secrets", ".penhan/keys/aes.key", ".penhan/vault-token"} {
		if !fileExists(t, s, want) {
			t.Errorf("expected %s to exist in the safe", want)
		}
	}
	if fileExists(t, s, ".penhan/state.json") {
		t.Error("safes must not carry a state file any more")
	}

	cfg, err := config.Load(filepath.Join(s.Dir, "penhan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend.Vault.BasePath != s.Name {
		t.Errorf("base_path = %q, want %q", cfg.Backend.Vault.BasePath, s.Name)
	}

	gitignore, err := os.ReadFile(filepath.Join(filepath.Dir(s.Dir), ".gitignore"))
	if err != nil {
		t.Fatalf("add must write a .gitignore in the project dir: %v", err)
	}
	for _, want := range []string{"secrets/*.yaml", ".penhan/keys/", ".penhan/vault-token"} {
		if !strings.Contains(string(gitignore), want) {
			t.Errorf(".gitignore missing %q:\n%s", want, gitignore)
		}
	}
}

func TestAddFileBackend(t *testing.T) {
	s := newFileSafe(t)

	cfg, err := config.Load(filepath.Join(s.Dir, "penhan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Backend.Type != "file" {
		t.Errorf("backend type = %q, want file", cfg.Backend.Type)
	}
	if !fileExists(t, s, cfg.Backend.File.Path) {
		t.Errorf("remote dir %s should be created", cfg.Backend.File.Path)
	}
	if fileExists(t, s, ".penhan/vault-token") {
		t.Error("file backend must not write a vault token")
	}
}

func TestAddRejectsDuplicate(t *testing.T) {
	dir := newProject(t)
	args := []string{"add", "dup",
		"--encryption=aes", "--backend=vault",
		"--vault-addr=" + vaultAddr, "--vault-token=" + vaultToken,
	}
	stdout, stderr, code := runPenhan(t, dir, args...)
	if code != 0 {
		t.Fatalf("first add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	stdout, stderr, code = runPenhan(t, dir, args...)
	if code == 0 {
		t.Fatal("second add must fail")
	}
	if !strings.Contains(stdout+stderr, "already") {
		t.Errorf("error should say the safe already exists: %s%s", stdout, stderr)
	}
}

func TestAddRejectsSchemelessVaultAddress(t *testing.T) {
	dir := newProject(t)
	stdout, stderr, code := runPenhan(t, dir, "add", "noscheme",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr=0.0.0.0:8200",
		"--vault-token="+vaultToken,
	)
	if code == 0 {
		t.Fatalf("add must reject a scheme-less vault address, got success: %s", stdout)
	}
	if !strings.Contains(stdout+stderr, "address") {
		t.Errorf("error should mention the address: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestAddNonTTYMissingFlags(t *testing.T) {
	dir := newProject(t)

	stdout, stderr, code := runPenhan(t, dir, "add")
	if code == 0 {
		t.Fatal("non-TTY add with missing flags must fail")
	}
	output := stdout + stderr
	for _, want := range []string{"non-interactive", "safe name", "--encryption", "--backend"} {
		if !strings.Contains(output, want) {
			t.Errorf("error should mention %q: %s", want, output)
		}
	}
}

func TestAddInvalidValues(t *testing.T) {
	dir := newProject(t)

	stdout, stderr, code := runPenhan(t, dir, "add", "bad", "--encryption=rot13", "--backend=vault",
		"--vault-addr="+vaultAddr, "--vault-token="+vaultToken)
	if code == 0 || !strings.Contains(stdout+stderr, "rot13") {
		t.Errorf("add must reject invalid encryption and name it: code=%d %s%s", code, stdout, stderr)
	}

	stdout, stderr, code = runPenhan(t, dir, "add", "bad", "--encryption=aes", "--backend=dynamo")
	if code == 0 || !strings.Contains(stdout+stderr, "dynamo") {
		t.Errorf("add must reject invalid backend and name it: code=%d %s%s", code, stdout, stderr)
	}

	stdout, stderr, code = runPenhan(t, dir, "add", "bad name!", "--encryption=aes", "--backend=file")
	if code == 0 {
		t.Errorf("add must reject an invalid safe name: %s%s", stdout, stderr)
	}
}

func TestAddVaultTokenFile(t *testing.T) {
	dir := newProject(t)

	tokenPath := filepath.Join(dir, "test-token")
	if err := os.WriteFile(tokenPath, []byte("file-token-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, dir, "add", "tokfile",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token-file="+tokenPath,
	)
	if code != 0 {
		t.Fatalf("add with --vault-token-file failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if strings.Contains(stderr, "Warning") {
		t.Errorf("token file flow must not warn: %s", stderr)
	}

	data, err := os.ReadFile(filepath.Join(dir, "tokfile", ".penhan", "vault-token"))
	if err != nil {
		t.Fatalf("vault-token not created: %v", err)
	}
	if string(data) != "file-token-value" {
		t.Errorf("vault-token content = %q, want %q", string(data), "file-token-value")
	}
}

func TestAddVaultTokenFlagWarns(t *testing.T) {
	dir := newProject(t)
	_, stderr, code := runPenhan(t, dir, "add", "tokflag",
		"--encryption=aes", "--backend=vault",
		"--vault-addr="+vaultAddr, "--vault-token="+vaultToken,
	)
	if code != 0 {
		t.Fatalf("add failed: %s", stderr)
	}
	if !strings.Contains(stderr, "Warning") {
		t.Errorf("--vault-token should warn on stderr, got: %s", stderr)
	}
}
