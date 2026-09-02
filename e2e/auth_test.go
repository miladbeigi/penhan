//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitAuthVariations covers the two supported ways to hand init a Vault
// token in non-interactive mode. Both must emit the runtime token file and
// leave a project that can actually talk to Vault (verified with a push).
func TestInitAuthVariations(t *testing.T) {
	vault := startVault(t)

	t.Run("vault_token_flag", func(t *testing.T) {
		dir := newProject(t)
		stdout, stderr, code := run(t, dir, "", "init",
			"--encryption=aes",
			"--backend=vault",
			"--vault-addr="+vault.addr,
			"--vault-token="+vault.token,
		)
		requireSuccess(t, "init", stdout, stderr, code)

		// The token flag is discouraged; the CLI must say so.
		if !strings.Contains(stderr, "Warning") {
			t.Errorf("init --vault-token should warn on stderr, got:\n%s", stderr)
		}
		requireFile(t, dir, ".penhan/vault-token")

		assertPushWorks(t, dir, "authflag")
	})

	t.Run("vault_token_file", func(t *testing.T) {
		dir := newProject(t)
		tokenFile := filepath.Join(dir, "test-token")
		if err := os.WriteFile(tokenFile, []byte(vault.token+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		stdout, stderr, code := run(t, dir, "", "init",
			"--encryption=aes",
			"--backend=vault",
			"--vault-addr="+vault.addr,
			"--vault-token-file="+tokenFile,
		)
		requireSuccess(t, "init", stdout, stderr, code)
		if strings.Contains(stderr, "Warning") {
			t.Errorf("token file flow must not warn, got:\n%s", stderr)
		}
		requireFile(t, dir, ".penhan/vault-token")

		assertPushWorks(t, dir, "authfile")
	})
}

// assertPushWorks pushes a secret in dir and confirms push completed —
// proving the saved credentials authenticate against Vault.
func assertPushWorks(t *testing.T, dir, secretName string) {
	t.Helper()
	writeFile(t, dir, "secrets/"+secretName+".yaml", "key: value\n")

	stdout, stderr, code := run(t, dir, "y\n", "push")
	requireSuccess(t, "push", stdout, stderr, code)
	if !strings.Contains(stdout, "Push complete") {
		t.Errorf("push output missing 'Push complete':\n%s", stdout)
	}
}
