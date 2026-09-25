package backends

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testEncryptor is a trivial XOR-based encryptor for testing.
type testEncryptor struct {
	key byte
}

func (e *testEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	out := make([]byte, len(plaintext))
	for i, b := range plaintext {
		out[i] = b ^ e.key
	}
	return out, nil
}

func (e *testEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	return e.Encrypt(ciphertext)
}

func TestNewFileProvider(t *testing.T) {
	if _, err := NewFileProvider("", &testEncryptor{0xAA}); err == nil {
		t.Error("expected error for missing directory")
	}
	if _, err := NewFileProvider(t.TempDir(), nil); err == nil {
		t.Error("expected error for missing encryptor")
	}

	dir := filepath.Join(t.TempDir(), "nested", "remote")
	if _, err := NewFileProvider(dir, &testEncryptor{0xAA}); err != nil {
		t.Fatalf("NewFileProvider() error = %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("remote directory should be created: %v", err)
	}
}

func TestFileProviderPushPull(t *testing.T) {
	dir := t.TempDir()
	p, err := NewFileProvider(dir, &testEncryptor{0xAA})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("push and pull", func(t *testing.T) {
		content := []byte(`{"key":"value"}`)
		if err := p.Push(content, "myapp/token"); err != nil {
			t.Fatalf("Push() error = %v", err)
		}

		got, err := p.Pull("myapp/token")
		if err != nil {
			t.Fatalf("Pull() error = %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("Pull() = %q, want %q", got, content)
		}
	})

	t.Run("stored encrypted", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join(dir, "myapp", "token.enc"))
		if err != nil {
			t.Fatalf("expected encrypted file: %v", err)
		}
		if strings.Contains(string(raw), "value") {
			t.Error("file backend must not store plaintext")
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		if err := p.Push([]byte(`{"key":"new"}`), "myapp/token"); err != nil {
			t.Fatal(err)
		}
		got, err := p.Pull("myapp/token")
		if err != nil || string(got) != `{"key":"new"}` {
			t.Errorf("Pull() = %q, %v", got, err)
		}
	})

	t.Run("pull missing", func(t *testing.T) {
		_, err := p.Pull("nope")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Pull() error = %v, want ErrNotFound", err)
		}
	})
}
