//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
)

// vaultData reads a secret's data at the given KV v2 data path
// (e.g. "secret/data/db"). Returns nil when the secret is absent
// or its latest version is deleted.
func vaultData(t *testing.T, v vaultEnv, path string) map[string]interface{} {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	cfg.Address = v.addr
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		t.Fatalf("vault client: %v", err)
	}
	client.SetToken(v.token)
	secret, err := client.Logical().Read(path)
	if err != nil {
		t.Fatalf("vault read %s: %v", path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil
	}
	data, _ := secret.Data["data"].(map[string]interface{})
	return data
}

// writeTokenFile drops a token file in dir and returns its path, for
// commands that accept --vault-token-file.
func writeTokenFile(t *testing.T, dir string, v vaultEnv) string {
	t.Helper()
	tokenFile := filepath.Join(dir, "test-token")
	if err := os.WriteFile(tokenFile, []byte(v.token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return tokenFile
}

// TestMultipleSafesIsolateBasePaths verifies safes push to distinct Vault
// base paths and both remain independently reachable.
func TestMultipleSafesIsolateBasePaths(t *testing.T) {
	vault := startVault(t)
	dir := newProject(t)
	tokenFile := writeTokenFile(t, dir, vault)

	for _, name := range []string{"alpha", "beta"} {
		stdout, stderr, code := run(t, dir, "", "safe", "add", name,
			"--encryption=aes",
			"--backend=vault",
			"--vault-addr="+vault.addr,
			"--vault-token-file="+tokenFile,
		)
		requireSuccess(t, "safe add "+name, stdout, stderr, code)
	}

	stdout, stderr, code := run(t, dir, "", "safe", "list")
	requireSuccess(t, "safe list", stdout, stderr, code)
	if !strings.Contains(stdout, "alpha") || !strings.Contains(stdout, "beta") {
		t.Errorf("safe list should show both safes:\n%s", stdout)
	}

	for _, safe := range []string{"alpha", "beta"} {
		safeDir := filepath.Join(dir, safe)
		writeFile(t, safeDir, "secrets/db.yaml", "key: value\n")

		stdout, stderr, code := run(t, safeDir, "y\n", "push")
		requireSuccess(t, "push in "+safe, stdout, stderr, code)

		if data := vaultData(t, vault, "secret/data/"+safe+"/db"); data == nil {
			t.Errorf("secret missing at secret/data/%s/db", safe)
		}
	}

	// Each safe's own Vault base path must not contain the other's secret.
	if data := vaultData(t, vault, "secret/data/alpha/db"); data == nil || data["key"] != "value" {
		t.Errorf("alpha base path should hold alpha's secret, got: %v", data)
	}
	if data := vaultData(t, vault, "secret/data/beta/db"); data == nil || data["key"] != "value" {
		t.Errorf("beta base path should hold beta's secret, got: %v", data)
	}
}
