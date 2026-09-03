package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
)

// fakeBackend is an in-memory Provider that records writes.
type fakeBackend struct {
	store  map[string][]byte
	pushed []string
}

func newFakeBackend() *fakeBackend { return &fakeBackend{store: map[string][]byte{}} }

func (f *fakeBackend) Push(content []byte, path string) error {
	f.store[path] = content
	f.pushed = append(f.pushed, path)
	return nil
}

func (f *fakeBackend) Pull(path string) ([]byte, error) {
	c, ok := f.store[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", backends.ErrNotFound, path)
	}
	return c, nil
}

func (f *fakeBackend) List(string) ([]string, error)     { return nil, nil }
func (f *fakeBackend) Delete(path string) error          { delete(f.store, path); return nil }
func (f *fakeBackend) Setup(backends.SetupOptions) error { return nil }
func (f *fakeBackend) IsInitialized() bool               { return true }

// newTestSafe creates a safe on disk with an AES key, chdirs into it, and
// returns its config plus provider.
func newTestSafe(t *testing.T) (*config.Config, crypto.Provider) {
	t.Helper()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.MkdirAll(filepath.Join(".penhan", "keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("secrets", 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Encryption: config.EncryptionConfig{Method: "aes", AES: config.AESConfig{KeyPath: ".penhan/keys/aes.key"}},
		Secrets:    config.SecretsConfig{Path: "secrets/", Format: "yaml"},
	}
	provider, err := newCryptoProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, provider
}

func writeSecretFile(t *testing.T, rel, content string) {
	t.Helper()
	p := filepath.Join("secrets", rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func statusOf(t *testing.T, results []checkResult, path string) string {
	t.Helper()
	for _, r := range results {
		if r.Secret.Path == path {
			return r.Status
		}
	}
	t.Fatalf("no result for %s in %v", path, results)
	return ""
}

func TestCheckClassifiesSecrets(t *testing.T) {
	cfg, provider := newTestSafe(t)
	backend := newFakeBackend()

	writeSecretFile(t, "fresh.yaml", "k: v\n")
	writeSecretFile(t, "same.yaml", "k: v\n")
	writeSecretFile(t, "apps/drift.yaml", "k: local\n")
	backend.store["same"] = []byte(`{"k":"v"}`)
	backend.store["apps/drift"] = []byte(`{"k":"remote"}`)

	results, err := checkSecrets(cfg, provider, backend)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if got := statusOf(t, results, "fresh"); got != StatusNew {
		t.Errorf("fresh = %s, want %s", got, StatusNew)
	}
	if got := statusOf(t, results, "same"); got != StatusUnchanged {
		t.Errorf("same = %s, want %s", got, StatusUnchanged)
	}
	if got := statusOf(t, results, "apps/drift"); got != StatusChanged {
		t.Errorf("apps/drift = %s, want %s", got, StatusChanged)
	}
	if len(backend.pushed) != 0 {
		t.Errorf("check must not write to the backend, pushed %v", backend.pushed)
	}
}

func TestCheckIgnoresKeyOrderAndWhitespace(t *testing.T) {
	cfg, provider := newTestSafe(t)
	backend := newFakeBackend()

	writeSecretFile(t, "db.yaml", "b: 2\na: 1\n")
	backend.store["db"] = []byte("{ \"b\": \"2\",\n \"a\": \"1\" }")

	results, err := checkSecrets(cfg, provider, backend)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, results, "db"); got != StatusUnchanged {
		t.Errorf("db = %s, want %s", got, StatusUnchanged)
	}
}

func TestCheckReadsEncryptedOnlySecrets(t *testing.T) {
	cfg, provider := newTestSafe(t)
	backend := newFakeBackend()

	enc, err := provider.Encrypt([]byte("k: v\n"))
	if err != nil {
		t.Fatal(err)
	}
	writeSecretFile(t, "sealed.yaml.enc", string(enc))

	results, err := checkSecrets(cfg, provider, backend)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, results, "sealed"); got != StatusNew {
		t.Errorf("sealed = %s, want %s", got, StatusNew)
	}
}

func TestCheckFailsOnBackendError(t *testing.T) {
	cfg, provider := newTestSafe(t)
	writeSecretFile(t, "db.yaml", "k: v\n")

	_, err := checkSecrets(cfg, provider, &failingBackend{})
	if err == nil {
		t.Fatal("expected error when the backend is unreachable")
	}
}

type failingBackend struct{ fakeBackend }

func (f *failingBackend) Pull(string) ([]byte, error) {
	return nil, fmt.Errorf("connection refused")
}

func TestCommandSet(t *testing.T) {
	want := map[string]bool{"add": true, "check": true, "push": true, "encrypt": true, "decrypt": true, "version": true}
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		if !want[c.Name()] {
			t.Errorf("unexpected command registered: %s", c.Name())
		}
		delete(want, c.Name())
	}
	for name := range want {
		t.Errorf("command %s is not registered", name)
	}
}
