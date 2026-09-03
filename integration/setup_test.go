//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	// vaultAddr is the address of the test Vault as seen from the host
	// (docker-compose maps host port 18200 to avoid clashing with a local Vault on 8200).
	vaultAddr = "http://127.0.0.1:18200"
	// vaultInContainerAddr is the address used when exec-ing the vault CLI inside the container.
	vaultInContainerAddr = "http://127.0.0.1:8200"
	vaultToken           = "penhan-test-token"
	mountPath            = "secret"
	vaultContainer       = "penhan-vault"
)

var penhanBinary string

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "integration setup failed: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.Remove(penhanBinary)
	os.Exit(code)
}

func setup() error {
	if err := waitForVault(30 * time.Second); err != nil {
		return fmt.Errorf("vault not ready: %w", err)
	}
	if err := resetMount(); err != nil {
		return fmt.Errorf("reset KV v2 mount: %w", err)
	}
	bin, err := buildBinary()
	if err != nil {
		return fmt.Errorf("build binary: %w", err)
	}
	penhanBinary = bin
	return nil
}

func waitForVault(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := httpGet(vaultAddr + "/v1/sys/health")
		if err == nil && status == http.StatusOK {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s", timeout)
}

// resetMount recreates the KV v2 mount so every run starts empty.
func resetMount() error {
	_, _ = vaultDockerExec("secrets", "disable", mountPath+"/")
	out, err := vaultDockerExec("secrets", "enable", "-path="+mountPath, "-version=2", "kv")
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

func buildBinary() (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp("", "penhan-it-bin-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(tmpDir, "penhan")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/penhan")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build: %w: %s", err, out)
	}
	return bin, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// newProject returns a fresh empty directory: the parent in which safes are created.
func newProject(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// safeNameFor derives a Vault-safe, per-test unique safe name so tests that
// share the compose Vault never collide on base paths.
func safeNameFor(t *testing.T) string {
	t.Helper()
	name := unsafeChars.ReplaceAllString(t.Name(), "-")
	if len(name) > 60 {
		name = name[len(name)-60:]
	}
	return strings.Trim(name, "-")
}

// safe is one created safe: its directory and its Vault base path.
type safe struct {
	Dir  string
	Name string
}

// newVaultSafe creates a Vault-backed safe named after the test inside a
// fresh project directory.
func newVaultSafe(t *testing.T) safe {
	t.Helper()
	dir := newProject(t)
	name := safeNameFor(t)
	stdout, stderr, code := runPenhan(t, dir, "add", name,
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+vaultAddr,
		"--vault-token="+vaultToken,
	)
	if code != 0 {
		t.Fatalf("add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	return safe{Dir: filepath.Join(dir, name), Name: name}
}

// newFileSafe creates a safe backed by the encrypted file backend.
func newFileSafe(t *testing.T) safe {
	t.Helper()
	dir := newProject(t)
	name := safeNameFor(t)
	stdout, stderr, code := runPenhan(t, dir, "add", name,
		"--encryption=aes",
		"--backend=file",
	)
	if code != 0 {
		t.Fatalf("add failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	return safe{Dir: filepath.Join(dir, name), Name: name}
}

// vaultPath returns the KV path (without the data/ prefix) for a secret in this safe.
func (s safe) vaultPath(secret string) string {
	return mountPath + "/" + s.Name + "/" + secret
}

func runPenhan(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	return runPenhanWithStdin(t, dir, nil, args...)
}

func runPenhanWithStdin(t *testing.T, dir string, stdin []byte, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, penhanBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"VAULT_ADDR="+vaultAddr,
		"VAULT_TOKEN="+vaultToken,
	)
	// Stdin is always a pipe so every command runs in non-TTY mode.
	cmd.Stdin = strings.NewReader(string(stdin))
	var outBuf, errBuf safeBuffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.Sys().(syscall.WaitStatus).ExitStatus()
		}
		t.Fatalf("run penhan %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}

type safeBuffer struct{ b []byte }

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}

func (s *safeBuffer) String() string { return string(s.b) }

// writeSecret writes a secret file at secrets/<rel> inside the safe.
func writeSecret(t *testing.T, s safe, rel, content string) {
	t.Helper()
	p := filepath.Join(s.Dir, "secrets", rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fileExists(t *testing.T, s safe, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(s.Dir, rel))
	return err == nil
}

func mustPush(t *testing.T, s safe) string {
	t.Helper()
	stdout, stderr, code := runPenhan(t, s.Dir, "push")
	if code != 0 {
		t.Fatalf("push failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	return stdout
}

func mustCheck(t *testing.T, s safe) string {
	t.Helper()
	stdout, stderr, code := runPenhan(t, s.Dir, "check")
	if code != 0 {
		t.Fatalf("check failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	return stdout
}

func vaultCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return vaultDockerExec(args...)
}

// vaultDockerExec runs a vault CLI subcommand inside the test container.
func vaultDockerExec(args ...string) (string, error) {
	dockerArgs := append([]string{"exec", "-e", "VAULT_ADDR=" + vaultInContainerAddr, "-e", "VAULT_TOKEN=" + vaultToken, vaultContainer, "vault"}, args...)
	cmd := exec.Command("docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func httpGet(url string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

// vaultPut writes key=value pairs at the safe's path, simulating an edit made outside penhan.
func vaultPut(t *testing.T, s safe, secret string, kv ...string) {
	t.Helper()
	args := append([]string{"kv", "put", s.vaultPath(secret)}, kv...)
	if out, err := vaultCmd(t, args...); err != nil {
		t.Fatalf("vault kv put: %v: %s", err, out)
	}
}

// readVaultData returns the secret's data, or nil when Vault has nothing there.
func readVaultData(t *testing.T, s safe, secret string) map[string]interface{} {
	t.Helper()
	out, err := vaultCmd(t, "kv", "get", "-format=json", s.vaultPath(secret))
	if err != nil {
		if strings.Contains(out, "No value found") {
			return nil
		}
		t.Fatalf("vault kv get %s: %v: %s", secret, err, out)
	}
	var parsed struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse vault json: %v", err)
	}
	return parsed.Data.Data
}
