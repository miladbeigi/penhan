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

// newProject returns a fresh empty directory for a scenario. Nothing is
// pre-created: init must set up the whole structure, as in real usage.
func newProject(t *testing.T) string {
	t.Helper()
	return t.TempDir()
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

type safeBuffer struct {
	b []byte
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}

func (s *safeBuffer) String() string {
	return string(s.b)
}

func requireSuccess(t *testing.T, name, stdout, stderr string, code int) {
	t.Helper()
	if code != 0 {
		t.Fatalf("%s failed: exit=%d\nstdout:\n%s\nstderr:\n%s", name, code, stdout, stderr)
	}
}

func requireFailure(t *testing.T, name, stdout, stderr string, code int) {
	t.Helper()
	if code == 0 {
		t.Fatalf("%s should have failed, but exited 0\nstdout:\n%s\nstderr:\n%s", name, stdout, stderr)
	}
}
