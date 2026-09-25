package commands

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/prompt"
)

func TestAppendGitignore_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	entries := []string{"secrets/", ".penhan/keys/", ".penhan/vault-token"}
	if err := appendGitignore(entries); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	for i, want := range entries {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
}

func TestAppendGitignore_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	existing := "node_modules/\n.env\n"
	if err := os.WriteFile(".gitignore", []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{"secrets/", ".penhan/keys/"}
	if err := appendGitignore(entries); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "node_modules/" || lines[1] != ".env" {
		t.Errorf("existing entries modified: %v", lines[:2])
	}
	if lines[2] != "secrets/" || lines[3] != ".penhan/keys/" {
		t.Errorf("new entries wrong: %v", lines[2:])
	}
}

func TestAppendGitignore_NoDuplicates(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	existing := "secrets/\n.penhan/keys/\n"
	if err := os.WriteFile(".gitignore", []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{"secrets/", ".penhan/keys/", ".penhan/vault-token"}
	if err := appendGitignore(entries); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (2 existing + 1 new), got %d: %v", len(lines), lines)
	}
	if lines[2] != ".penhan/vault-token" {
		t.Errorf("last line = %q, want %q", lines[2], ".penhan/vault-token")
	}
}

func TestAppendGitignore_EmptyEntries(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	existing := "secrets/\n"
	if err := os.WriteFile(".gitignore", []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendGitignore([]string{"secrets/"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (no change), got %d: %v", len(lines), lines)
	}
}

func TestAppendGitignore_PreservesTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	existing := "node_modules/\n"
	if err := os.WriteFile(".gitignore", []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{"secrets/"}
	if err := appendGitignore(entries); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasSuffix(string(data), "\n") {
		t.Error("expected trailing newline")
	}
}

func TestAppendGitignore_SubdirectoryPath(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	entries := []string{"src/secrets/", ".penhan/keys/"}
	if err := appendGitignore(entries); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "src/secrets/" {
		t.Errorf("line 0 = %q, want %q", lines[0], "src/secrets/")
	}
}

func TestAppendGitignore_DirWithTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	// Add without trailing slash, try to add with — should not duplicate
	existing := ".penhan/keys\n"
	if err := os.WriteFile(".gitignore", []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []string{".penhan/keys/"}
	if err := appendGitignore(entries); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}

	// Different strings — both should be present (user's choice)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (different strings are distinct), got %d: %v", len(lines), lines)
	}
}

func TestAppendGitignore_MissingDir(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	// File doesn't exist yet — creates it
	if err := appendGitignore([]string{"secrets/"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(".gitignore"); os.IsNotExist(err) {
		t.Error("expected .gitignore to be created")
	}
}

func TestAppendGitignore_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	// Symlink to non-existent target — ReadFile will fail with not-exist
	if err := os.Symlink(filepath.Join(dir, "nonexistent"), ".gitignore"); err != nil {
		t.Fatal(err)
	}

	// Should treat as not-existing and create fresh
	if err := appendGitignore([]string{"secrets/"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(".gitignore"); os.IsNotExist(err) {
		t.Error("expected .gitignore to be created")
	}
}

// Patterns containing a slash are anchored to the directory holding the
// .gitignore. Since add writes the project-root .gitignore, every entry
// must be prefixed with the safe directory or git never matches it.
func TestGitignoreEntries_PrefixedWithSafeDir(t *testing.T) {
	got := gitignoreEntries("vault", "vault")
	want := []string{
		"vault/secrets/**/*.yaml",
		"vault/secrets/**/*.yml",
		"vault/secrets/**/*.json",
		"vault/.penhan/keys/",
		"vault/.penhan/vault-token",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGitignoreEntries_FileBackendHasNoToken(t *testing.T) {
	for _, e := range gitignoreEntries("myapp", "file") {
		if strings.Contains(e, "vault-token") {
			t.Errorf("file backend must not ignore a vault token, got %q", e)
		}
		if !strings.HasPrefix(e, "myapp/") {
			t.Errorf("entry %q is not prefixed with the safe dir", e)
		}
	}
}

func TestCreateSafe_KeySetupFailureRollsBackNewDirectory(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origGenerate := generateKey
	generateKey = func(string, string) error { return errors.New("key setup failed") }
	t.Cleanup(func() { generateKey = origGenerate })

	safeName := "myapp"
	err = createSafe(&prompt.InitAnswers{
		SafeName:   safeName,
		Encryption: "aes",
		Backend:    "file",
	})
	if err == nil {
		t.Fatal("expected createSafe to fail when key setup fails")
	}
	if _, statErr := os.Stat(filepath.Join(tmp, safeName)); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to be removed after failed add, stat err = %v", safeName, statErr)
	}
}

func TestCreateSafe_KeySetupFailureKeepsPreExistingDirectory(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origGenerate := generateKey
	generateKey = func(string, string) error { return errors.New("key setup failed") }
	t.Cleanup(func() { generateKey = origGenerate })

	safeName := "myapp"
	if err := os.Mkdir(filepath.Join(tmp, safeName), 0o755); err != nil {
		t.Fatal(err)
	}

	err = createSafe(&prompt.InitAnswers{
		SafeName:   safeName,
		Encryption: "aes",
		Backend:    "file",
	})
	if err == nil {
		t.Fatal("expected createSafe to fail when key setup fails")
	}
	if _, statErr := os.Stat(filepath.Join(tmp, safeName)); statErr != nil {
		t.Fatalf("expected pre-existing directory %q to be kept, stat err = %v", safeName, statErr)
	}
}

func TestCreateSafe_KeySetupFailureDoesNotUpdateGitignore(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origGenerate := generateKey
	generateKey = func(string, string) error { return errors.New("key setup failed") }
	t.Cleanup(func() { generateKey = origGenerate })

	before := "node_modules/\n"
	if err := os.WriteFile(".gitignore", []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	err = createSafe(&prompt.InitAnswers{
		SafeName:   "myapp",
		Encryption: "aes",
		Backend:    "file",
	})
	if err == nil {
		t.Fatal("expected createSafe to fail when key setup fails")
	}

	after, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Fatalf("expected .gitignore to remain unchanged, got %q, want %q", string(after), before)
	}
}

func TestStdinIsTTY_FalseForDevNull(t *testing.T) {
	orig := os.Stdin
	t.Cleanup(func() { os.Stdin = orig })

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devNull.Close() })

	os.Stdin = devNull
	if stdinIsTTY() {
		t.Fatalf("stdinIsTTY() = true, want false for %s", os.DevNull)
	}
}

func TestStdinIsTTY_FalseForPipe(t *testing.T) {
	orig := os.Stdin
	t.Cleanup(func() { os.Stdin = orig })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	os.Stdin = r
	if stdinIsTTY() {
		t.Fatal("stdinIsTTY() = true, want false for pipe stdin")
	}
}

const twoContextKubeconfig = `apiVersion: v1
kind: Config
current-context: dev
clusters:
- {name: c, cluster: {server: "https://127.0.0.1:6443"}}
users:
- {name: u, user: {token: t}}
contexts:
- {name: dev, context: {cluster: c, user: u}}
- {name: prod, context: {cluster: c, user: u}}
`

func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Tests run with a pipe on stdin, so resolveKubeContext never prompts here.
func TestResolveKubeContext(t *testing.T) {
	kubeconfig := writeKubeconfig(t, twoContextKubeconfig)

	if got, err := resolveKubeContext(kubeconfig, ""); err != nil || got != "dev" {
		t.Errorf("default = %q, %v; want the current context dev", got, err)
	}
	if got, err := resolveKubeContext(kubeconfig, "prod"); err != nil || got != "prod" {
		t.Errorf("explicit = %q, %v; want prod", got, err)
	}
	if _, err := resolveKubeContext(kubeconfig, "staging"); err == nil || !strings.Contains(err.Error(), "staging") {
		t.Errorf("unknown context should fail and name it, got %v", err)
	}

	noCurrent := writeKubeconfig(t, strings.Replace(twoContextKubeconfig, "current-context: dev\n", "", 1))
	if _, err := resolveKubeContext(noCurrent, ""); err == nil || !strings.Contains(err.Error(), "--kube-context") {
		t.Errorf("no current context should ask for --kube-context, got %v", err)
	}
}

func TestMissingFlags_Kubernetes(t *testing.T) {
	missing := missingFlags(&prompt.InitAnswers{SafeName: "a", Encryption: "aes", Backend: "kubernetes"})
	if len(missing) != 1 || missing[0] != "--kube-namespace" {
		t.Errorf("missingFlags() = %v, want only --kube-namespace", missing)
	}
}

func TestCreateSafe_Kubernetes(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	err := createSafe(&prompt.InitAnswers{
		SafeName:      "myapp",
		Encryption:    "aes",
		Backend:       "kubernetes",
		Kubeconfig:    "/home/me/.kube/config",
		KubeContext:   "prod",
		KubeNamespace: "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(filepath.Join("myapp", "penhan.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := config.KubernetesConfig{Kubeconfig: "/home/me/.kube/config", Context: "prod", Namespace: "payments", Safe: "myapp"}
	if cfg.Backend.Type != "kubernetes" || cfg.Backend.Kubernetes != want {
		t.Errorf("backend = %+v, want kubernetes %+v", cfg.Backend, want)
	}
	if _, err := os.Stat(filepath.Join("myapp", ".penhan", "vault-token")); !os.IsNotExist(err) {
		t.Error("kubernetes backend must not write a vault token")
	}
}

// Ask git itself: plaintext at any depth under secrets/ and the keys must be
// ignored, while the encrypted copies and penhan.yaml must stay committable.
func TestGitignoreEntries_MatchedByGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := appendGitignore(gitignoreEntries("myapp", "vault")); err != nil {
		t.Fatal(err)
	}

	ignored := []string{
		"myapp/secrets/db.yaml",
		"myapp/secrets/db/password.yaml",
		"myapp/secrets/a/b/c/deep.json",
		"myapp/secrets/api.yml",
		"myapp/.penhan/keys/aes.key",
		"myapp/.penhan/vault-token",
	}
	tracked := []string{
		"myapp/secrets/db.yaml.enc",
		"myapp/secrets/db/password.yaml.enc",
		"myapp/penhan.yaml",
	}
	for _, p := range ignored {
		if err := exec.Command("git", "check-ignore", "-q", p).Run(); err != nil {
			t.Errorf("%s must be ignored by git", p)
		}
	}
	for _, p := range tracked {
		if err := exec.Command("git", "check-ignore", "-q", p).Run(); err == nil {
			t.Errorf("%s must not be ignored by git", p)
		}
	}
}
