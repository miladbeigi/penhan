package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	vaultAddr       = "http://127.0.0.1:8200"
	vaultToken      = "penhan-test-token"
	mountPath       = "secret"
	vaultContainer  = "penhan-vault"
	vaultImage      = "hashicorp/vault:1.17"
)

var (
	penhanBinary string
	projectDir   string
)

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Fprintf(os.Stderr, "integration setup failed: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() error {
	if err := waitForVault(30 * time.Second); err != nil {
		return fmt.Errorf("vault not ready: %w", err)
	}
	if err := enableKV2(); err != nil {
		return fmt.Errorf("enable KV v2: %w", err)
	}
	bin, err := buildBinary()
	if err != nil {
		return fmt.Errorf("build binary: %w", err)
	}
	penhanBinary = bin
	projectDir, err = os.MkdirTemp("", "penhan-it-*")
	if err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	return nil
}

func teardown() {
	if projectDir != "" {
		_ = os.RemoveAll(projectDir)
	}
	if penhanBinary != "" {
		_ = os.Remove(penhanBinary)
	}
}

func waitForVault(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := httpGet(vaultAddr + "/v1/sys/health")
		if err == nil && resp == 200 {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("vault not ready within %s", timeout)
}

func enableKV2() error {
	if out, err := vaultDockerExec("vault", "secrets", "enable", "-path="+mountPath, "-version=2", "kv"); err != nil {
		if !strings.Contains(string(out), "already") {
			return fmt.Errorf("enable kv: %w: %s", err, out)
		}
	}
	return nil
}

func buildBinary() (string, error) {
	tmpDir, err := os.MkdirTemp("", "penhan-bin-*")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(tmpDir, "penhan")
	root, err := findRepoRoot()
	if err != nil {
		return "", err
	}
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

func newProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"secrets", ".penhan/keys"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runPenhan(t *testing.T, dir string, args ...string) (string, string, int) {
	t.Helper()
	return runPenhanWithStdin(t, dir, nil, args...)
}

func runPenhanWithStdin(t *testing.T, dir string, stdin []byte, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, penhanBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"VAULT_ADDR="+vaultAddr,
		"VAULT_TOKEN="+vaultToken,
	)
	if stdin != nil {
		cmd.Stdin = strings.NewReader(string(stdin))
	}
	var outBuf, errBuf safeBuffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), ee.Sys().(syscall.WaitStatus).ExitStatus()
		}
		t.Fatalf("run penhan: %v", err)
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

func initProject(t *testing.T, dir string) {
	t.Helper()
	input := "aes\nvault\n" + vaultAddr + "\n" + vaultToken + "\n"
	stdout, stderr, code := runPenhanWithStdin(t, dir, []byte(input), "init")
	if code != 0 {
		t.Fatalf("init failed: code=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
}

func writeSecret(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "secrets", name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func vaultCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return vaultDockerExec(args...)
}

func vaultDockerExec(args ...string) (string, error) {
	dockerArgs := append([]string{"exec", "-e", "VAULT_ADDR=" + vaultAddr, "-e", "VAULT_TOKEN=" + vaultToken, vaultContainer}, args...)
	cmd := exec.Command("docker", dockerArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func httpGet(url string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func readVaultData(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	out, err := vaultCmd(t, "kv", "get", "-format=json", mountPath+"/"+path)
	if err != nil {
		t.Fatalf("read vault %s: %v: %s", path, err, out)
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
