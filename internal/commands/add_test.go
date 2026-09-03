package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
