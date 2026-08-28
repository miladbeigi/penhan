//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Nested secrets (field test findings H1/H2) ---

func TestPullRestoresNestedSecrets(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "apps/api-token", "nested-tok")
	addSecret(t, dir, "db", "flat-pass")
	mustPush(t, dir)

	// Fresh clone: no local secrets, no state.
	if err := os.RemoveAll(filepath.Join(dir, "secrets")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".penhan", "state.json")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "pull")
	if code != 0 {
		t.Fatalf("pull failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	for _, want := range []string{"secrets/db.yaml.enc", "secrets/apps/api-token.yaml.enc"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("expected %s restored: %v", want, err)
		}
	}

	stdout, stderr, code = runPenhan(t, dir, "decrypt", "secrets/apps/api-token.yaml.enc")
	if code != 0 {
		t.Fatalf("decrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	data, err := os.ReadFile(filepath.Join(dir, "secrets", "apps", "api-token.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "nested-tok") {
		t.Errorf("nested secret content mismatch: %s", data)
	}
}

func TestPlanSettlesAfterNestedPush(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "apps/api-token", "tok")
	mustPush(t, dir)

	stdout, stderr, code := runPenhan(t, dir, "plan")
	if code != 0 {
		t.Fatalf("plan failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "0 to add") {
		t.Errorf("plan should settle after pushing a nested secret: %s", stdout)
	}
}

func TestPullSurvivesRemovedSecret(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "doomed", "bye")
	addSecret(t, dir, "survivor", "hello")
	mustPush(t, dir)

	if _, _, code := runPenhan(t, dir, "remove", "secrets/doomed.yaml", "--force"); code != 0 {
		t.Fatal("remove failed")
	}

	if err := os.RemoveAll(filepath.Join(dir, "secrets")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ".penhan", "state.json")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "pull")
	if code != 0 {
		t.Fatalf("pull after remove failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets", "survivor.yaml.enc")); err != nil {
		t.Errorf("expected survivor restored: %v", err)
	}
}

// --- list robustness (findings M1/M2) ---

func TestListWorksWithoutStateFile(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "fresh", "v")

	if err := os.Remove(filepath.Join(dir, ".penhan", "state.json")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, dir, "list")
	if code != 0 {
		t.Fatalf("list without state failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "fresh.yaml") {
		t.Errorf("list missing fresh.yaml: %s", stdout)
	}
}

func TestListShowsEncryptedOnlySecrets(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "vaulted", "v")

	if _, _, code := runPenhan(t, dir, "encrypt", "secrets/vaulted.yaml"); code != 0 {
		t.Fatal("encrypt failed")
	}
	// Tolerate encrypt already having removed the plaintext.
	_ = os.Remove(filepath.Join(dir, "secrets", "vaulted.yaml"))

	stdout, stderr, code := runPenhan(t, dir, "list")
	if code != 0 {
		t.Fatalf("list failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "vaulted") {
		t.Errorf("list must show encrypted-only secrets: %s", stdout)
	}
}

// --- config validation and backend visibility (finding M4) ---

func TestInitRejectsSchemelessVaultAddress(t *testing.T) {
	dir := newProject(t)
	stdout, stderr, code := runPenhan(t, dir, "init",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr=0.0.0.0:8200",
		"--vault-token="+vaultToken,
	)
	if code == 0 {
		t.Fatalf("init must reject a scheme-less vault address, got success: %s", stdout)
	}
	if !strings.Contains(stdout+stderr, "address") {
		t.Errorf("error should mention the address: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestPlanWarnsWhenBackendUnreachable(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "offline", "v")

	cfgPath := filepath.Join(dir, "penhan.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(cfg), vaultAddr, "http://127.0.0.1:1", 1)
	if err := os.WriteFile(cfgPath, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, dir, "plan")
	if code != 0 {
		t.Fatalf("plan should still work offline: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stderr, "warning") {
		t.Errorf("plan must warn when the backend is unreachable: stderr=%s", stderr)
	}
}

// --- remote drift protection (finding M5) ---

func TestPushBlocksRemoteOnlyDrift(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	writeSecret(t, dir, "drift.yaml", "v: original\n")
	mustPush(t, dir)

	if out, err := vaultCmd(t, "kv", "put", mountPath+"/drift", "v=remote-edit"); err != nil {
		t.Fatalf("vault put: %v: %s", err, out)
	}

	// Local unchanged: pushing would clobber the remote edit.
	stdout, _, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push")
	if code == 0 {
		t.Errorf("push must not silently overwrite remote-only changes: %s", stdout)
	}
	data := readVaultData(t, "drift")
	if data["v"] != "remote-edit" {
		t.Errorf("remote edit must be preserved, got: %v", data)
	}

	// --force accepts the overwrite explicitly.
	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("y\n"), "push", "--force")
	if code != 0 {
		t.Fatalf("force push failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	data = readVaultData(t, "drift")
	if data["v"] != "original" {
		t.Errorf("force push should restore local value, got: %v", data)
	}
}

// --- encrypt/decrypt ergonomics (findings L1/L2) ---

func TestEncryptRemovesPlaintext(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "atrest", "v")

	stdout, stderr, code := runPenhan(t, dir, "encrypt", "secrets/atrest.yaml")
	if code != 0 {
		t.Fatalf("encrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets", "atrest.yaml.enc")); err != nil {
		t.Fatalf("encrypted file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets", "atrest.yaml")); !os.IsNotExist(err) {
		t.Errorf("encrypt must remove the plaintext file, got err=%v", err)
	}
}

func TestEncryptDecryptDefaultToSecretsDir(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)
	addSecret(t, dir, "one", "1")
	addSecret(t, dir, "sub/two", "2")

	stdout, stderr, code := runPenhan(t, dir, "encrypt")
	if code != 0 {
		t.Fatalf("no-arg encrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	for _, want := range []string{"secrets/one.yaml.enc", "secrets/sub/two.yaml.enc"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("no-arg encrypt should cover %s: %v", want, err)
		}
	}

	stdout, stderr, code = runPenhan(t, dir, "decrypt")
	if code != 0 {
		t.Fatalf("no-arg decrypt failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	for _, want := range []string{"secrets/one.yaml", "secrets/sub/two.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("no-arg decrypt should restore %s: %v", want, err)
		}
	}
}

// --- guard rails (findings L3/L4/L5) ---

func TestReinitRefusesExistingProject(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)

	stdout, stderr, code := runPenhan(t, dir, "init",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code == 0 {
		t.Fatalf("re-init must refuse to overwrite an existing project: %s", stdout)
	}
	if !strings.Contains(stdout+stderr, "already") {
		t.Errorf("error should say the project already exists: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestErrorsDoNotPrintUsage(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)

	stdout, stderr, code := runPenhan(t, dir, "remove", "secrets/ghost.yaml", "--force")
	if code == 0 {
		t.Fatal("removing a missing secret must fail")
	}
	if strings.Contains(stdout+stderr, "Usage:") {
		t.Errorf("runtime errors must not dump the usage block: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestAddWarnsOnEmptyValue(t *testing.T) {
	dir := newProject(t)
	initProject(t, dir)

	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte("\n"), "add", "blank")
	if code != 0 {
		t.Fatalf("add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(strings.ToLower(stdout+stderr), "warning") {
		t.Errorf("add must warn about an empty value: stdout=%s stderr=%s", stdout, stderr)
	}
}

func TestInitNonTTYMissingFlags(t *testing.T) {
	dir := newProject(t)

	// Simulate non-TTY by piping empty stdin with no flags
	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte(""), "init")
	if code == 0 {
		t.Fatal("non-TTY init with missing flags must fail")
	}
	output := stdout + stderr
	if !strings.Contains(output, "non-interactive") {
		t.Errorf("error should mention non-interactive mode: %s", output)
	}
	if !strings.Contains(output, "--encryption") {
		t.Errorf("error should list --encryption as missing: %s", output)
	}
}

func TestInitInvalidEncryption(t *testing.T) {
	dir := newProject(t)

	stdout, stderr, code := runPenhan(t, dir, "init",
		"--encryption=rot13",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code == 0 {
		t.Fatal("init must reject invalid encryption method")
	}
	if !strings.Contains(stdout+stderr, "rot13") {
		t.Errorf("error should mention the invalid value: %s", stdout)
	}
}

func TestInitInvalidBackend(t *testing.T) {
	dir := newProject(t)

	stdout, stderr, code := runPenhan(t, dir, "init",
		"--encryption=aes",
		"--backend=dynamo",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code == 0 {
		t.Fatal("init must reject invalid backend")
	}
	if !strings.Contains(stdout+stderr, "dynamo") {
		t.Errorf("error should mention the invalid backend: %s", stdout)
	}
}

func TestInitVaultTokenFile(t *testing.T) {
	dir := newProject(t)

	tokenPath := filepath.Join(dir, ".penhan", "test-token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte("file-token-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, dir, "init",
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token-file="+tokenPath,
	)
	if code != 0 {
		t.Fatalf("init with --vault-token-file failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}

	vaultTokenPath := filepath.Join(dir, ".penhan", "vault-token")
	data, err := os.ReadFile(vaultTokenPath)
	if err != nil {
		t.Fatalf("vault-token not created: %v", err)
	}
	if string(data) != "file-token-value" {
		t.Errorf("vault-token content = %q, want %q", string(data), "file-token-value")
	}
}
