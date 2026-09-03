//go:build e2e

package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMultipleSafesIsolateBasePaths verifies that safes created side by side
// push to distinct Vault base paths and never see each other's secrets.
func TestMultipleSafesIsolateBasePaths(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)

	for _, name := range []string{"alpha", "beta"} {
		safe := addSafe(t, dir, name, vault)
		writeFile(t, safe, "secrets/db.yaml", "owner: "+name+"\n")
		stdout, stderr, code := run(t, safe, "", "push")
		requireSuccess(t, "push in "+name, stdout, stderr, code)
	}

	for _, name := range []string{"alpha", "beta"} {
		data := vaultData(t, vault, "secret/data/"+name+"/db")
		if data == nil || data["owner"] != name {
			t.Errorf("secret/data/%s/db should hold %s's secret, got %v", name, name, data)
		}
	}
	if vaultData(t, vault, "secret/data/db") != nil {
		t.Error("nothing should be written outside a safe base path")
	}
}

// TestAddTokenFlagVariations covers the two ways to hand add a Vault token
// non-interactively. Both must produce a safe that can push.
func TestAddTokenFlagVariations(t *testing.T) {
	vault := startVault(t)

	t.Run("vault_token_flag", func(t *testing.T) {
		dir := newProject(t)
		stdout, stderr, code := run(t, dir, "", "add", "flag",
			"--encryption=aes",
			"--backend=vault",
			"--vault-addr="+vault.addr,
			"--vault-token="+vault.token,
		)
		requireSuccess(t, "add", stdout, stderr, code)
		if !strings.Contains(stderr, "Warning") {
			t.Errorf("add --vault-token should warn on stderr, got:\n%s", stderr)
		}
		assertPushWorks(t, filepath.Join(dir, "flag"))
	})

	t.Run("vault_token_file", func(t *testing.T) {
		dir := newProject(t)
		assertPushWorks(t, addSafe(t, dir, "file", vault))
	})
}

func assertPushWorks(t *testing.T, safe string) {
	t.Helper()
	requireFile(t, safe, ".penhan/vault-token")
	writeFile(t, safe, "secrets/auth.yaml", "key: value\n")
	stdout, stderr, code := run(t, safe, "", "push")
	requireSuccess(t, "push", stdout, stderr, code)
	requireContains(t, "push", stdout, "Push complete")
}

// TestAddNonInteractiveRequiresFlags verifies scripts get a clear error
// instead of a hung prompt.
func TestAddNonInteractiveRequiresFlags(t *testing.T) {
	dir := newProject(t)
	stdout, stderr, code := run(t, dir, "", "add")
	if code == 0 {
		t.Fatalf("add without flags on a pipe must fail:\n%s", stdout)
	}
	requireContains(t, "add", stdout+stderr, "non-interactive")
	requireContains(t, "add", stdout+stderr, "--encryption")
}
