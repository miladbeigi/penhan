//go:build e2e

// Package e2e runs the real penhan binary against a real Vault server.
// Each scenario starts with a throwaway Vault container and a fresh
// project directory, exercising the CLI exactly as a user would.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	vaultImage = "hashicorp/vault:1.21"
	vaultToken = "penhan-e2e-token"
)

var penhanBinary string

func TestMain(m *testing.M) {
	bin, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup failed: %v\n", err)
		os.Exit(1)
	}
	penhanBinary = bin
	code := m.Run()
	_ = os.Remove(bin)
	os.Exit(code)
}

func buildBinary() (string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp("", "penhan-e2e-bin-*")
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

// vaultEnv is one throwaway Vault server, live for the duration of a test.
type vaultEnv struct {
	addr  string
	token string
}

// startVault starts a dev-mode Vault container for this test and registers
// its termination as cleanup. Every test gets its own container: no shared
// state between scenarios, nothing survives the run.
func startVault(t *testing.T) vaultEnv {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        vaultImage,
		ExposedPorts: []string{"8200/tcp"},
		Env: map[string]string{
			"VAULT_DEV_ROOT_TOKEN_ID":  vaultToken,
			"VAULT_DEV_LISTEN_ADDRESS": "0.0.0.0:8200",
		},
		CapAdd: []string{"IPC_LOCK"},
		WaitingFor: wait.ForHTTP("/v1/sys/health").
			WithPort("8200/tcp").
			WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start vault container: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("vault container host: %v", err)
	}
	port, err := c.MappedPort(ctx, "8200/tcp")
	if err != nil {
		t.Fatalf("vault container port: %v", err)
	}
	return vaultEnv{addr: fmt.Sprintf("http://%s:%s", host, port.Port()), token: vaultToken}
}

func vaultClient(t *testing.T, v vaultEnv) *vaultapi.Client {
	t.Helper()
	cfg := vaultapi.DefaultConfig()
	cfg.Address = v.addr
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		t.Fatalf("vault client: %v", err)
	}
	client.SetToken(v.token)
	return client
}

// vaultData reads a secret's data at the given KV v2 data path
// (e.g. "secret/data/myapp/db"). Returns nil when the secret is absent.
func vaultData(t *testing.T, v vaultEnv, path string) map[string]interface{} {
	t.Helper()
	secret, err := vaultClient(t, v).Logical().Read(path)
	if err != nil {
		t.Fatalf("vault read %s: %v", path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil
	}
	data, _ := secret.Data["data"].(map[string]interface{})
	return data
}

// vaultWrite stores key-values at a KV v2 data path, simulating an edit made
// outside penhan.
func vaultWrite(t *testing.T, v vaultEnv, path string, kv map[string]interface{}) {
	t.Helper()
	if _, err := vaultClient(t, v).Logical().Write(path, map[string]interface{}{"data": kv}); err != nil {
		t.Fatalf("vault write %s: %v", path, err)
	}
}

// newProject returns a fresh empty directory in which safes are created.
func newProject(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// writeTokenFile drops a token file in dir and returns its path.
func writeTokenFile(t *testing.T, dir string, v vaultEnv) string {
	t.Helper()
	tokenFile := filepath.Join(dir, "test-token")
	if err := os.WriteFile(tokenFile, []byte(v.token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return tokenFile
}

// addSafe runs `penhan add <name>` non-interactively against v and returns
// the safe's directory.
func addSafe(t *testing.T, dir, name string, v vaultEnv) string {
	t.Helper()
	tokenFile := writeTokenFile(t, dir, v)
	stdout, stderr, code := run(t, dir, "", "add", name,
		"--encryption=aes",
		"--backend=vault",
		"--vault-addr="+v.addr,
		"--vault-token-file="+tokenFile,
	)
	requireSuccess(t, "add "+name, stdout, stderr, code)
	return filepath.Join(dir, name)
}

// run executes the penhan binary in dir with the given stdin and returns
// stdout, stderr, and the exit code. Stdin is always a pipe, so every
// command runs in the same deterministic non-TTY mode a script would get.
func run(t *testing.T, dir string, stdin string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, penhanBinary, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
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

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func requireFile(t *testing.T, dir, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
		t.Fatalf("expected %s to exist: %v", rel, err)
	}
}

func requireNoFile(t *testing.T, dir, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, got err=%v", rel, err)
	}
}

func requireSuccess(t *testing.T, name, stdout, stderr string, code int) {
	t.Helper()
	if code != 0 {
		t.Fatalf("%s failed: exit=%d\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
	}
}

func requireContains(t *testing.T, name, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Errorf("%s output missing %q:\n%s", name, want, output)
	}
}
