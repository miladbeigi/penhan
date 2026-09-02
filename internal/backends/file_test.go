package backends

import (
	"os"
	"path/filepath"
	"sort"
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

func TestFileProviderSetup(t *testing.T) {
	dir := t.TempDir()

	t.Run("success", func(t *testing.T) {
		p := NewFileProvider()
		err := p.Setup(SetupOptions{Dir: dir, Enc: &testEncryptor{0xAA}})
		if err != nil {
			t.Fatalf("Setup() error = %v", err)
		}
		if !p.IsInitialized() {
			t.Error("IsInitialized() = false, want true")
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		p := NewFileProvider()
		err := p.Setup(SetupOptions{Enc: &testEncryptor{0xAA}})
		if err == nil {
			t.Fatal("expected error for missing directory")
		}
	})

	t.Run("missing encryptor", func(t *testing.T) {
		p := NewFileProvider()
		err := p.Setup(SetupOptions{Dir: dir})
		if err == nil {
			t.Fatal("expected error for missing encryptor")
		}
	})
}

func TestFileProviderPushPull(t *testing.T) {
	dir := t.TempDir()
	p := NewFileProvider()
	if err := p.Setup(SetupOptions{Dir: dir, Enc: &testEncryptor{0xAA}}); err != nil {
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

	t.Run("ciphertext on disk is not plaintext", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join(dir, "myapp", "token.enc"))
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) == `{"key":"value"}` {
			t.Error("ciphertext on disk equals plaintext — no encryption applied")
		}
	})

	t.Run("pull nonexistent", func(t *testing.T) {
		_, err := p.Pull("nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent secret")
		}
	})

	t.Run("nested path", func(t *testing.T) {
		if err := p.Push([]byte("nested"), "a/b/c"); err != nil {
			t.Fatal(err)
		}
		got, err := p.Pull("a/b/c")
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "nested" {
			t.Errorf("got %q, want %q", got, "nested")
		}
	})
}

func TestFileProviderList(t *testing.T) {
	dir := t.TempDir()
	p := NewFileProvider()
	if err := p.Setup(SetupOptions{Dir: dir, Enc: &testEncryptor{0xAA}}); err != nil {
		t.Fatal(err)
	}

	t.Run("empty directory", func(t *testing.T) {
		paths, err := p.List("")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Errorf("List() = %v, want empty", paths)
		}
	})

	t.Run("flat and nested", func(t *testing.T) {
		if err := p.Push([]byte("a"), "alpha"); err != nil {
			t.Fatal(err)
		}
		if err := p.Push([]byte("b"), "beta"); err != nil {
			t.Fatal(err)
		}
		if err := p.Push([]byte("c"), "sub/gamma"); err != nil {
			t.Fatal(err)
		}
		if err := p.Push([]byte("d"), "sub/deep/delta"); err != nil {
			t.Fatal(err)
		}

		paths, err := p.List("")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(paths)
		want := []string{"alpha", "beta", "sub/deep/delta", "sub/gamma"}
		if !equalSlices(paths, want) {
			t.Errorf("List() = %v, want %v", paths, want)
		}
	})

	t.Run("list subdirectory", func(t *testing.T) {
		paths, err := p.List("sub")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(paths)
		want := []string{"sub/deep/delta", "sub/gamma"}
		if !equalSlices(paths, want) {
			t.Errorf("List(sub) = %v, want %v", paths, want)
		}
	})

	t.Run("list nonexistent", func(t *testing.T) {
		paths, err := p.List("nonexistent")
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) != 0 {
			t.Errorf("List(nonexistent) = %v, want empty", paths)
		}
	})
}

func TestFileProviderDelete(t *testing.T) {
	dir := t.TempDir()
	p := NewFileProvider()
	if err := p.Setup(SetupOptions{Dir: dir, Enc: &testEncryptor{0xAA}}); err != nil {
		t.Fatal(err)
	}

	t.Run("delete existing", func(t *testing.T) {
		if err := p.Push([]byte("x"), "toremove"); err != nil {
			t.Fatal(err)
		}
		if err := p.Delete("toremove"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if _, err := p.Pull("toremove"); err == nil {
			t.Error("expected error after delete")
		}
	})

	t.Run("delete nonexistent", func(t *testing.T) {
		err := p.Delete("never-existed")
		if err != nil {
			t.Fatalf("Delete(nonexistent) error = %v", err)
		}
	})
}

func TestFileProviderUninitialized(t *testing.T) {
	p := NewFileProvider()

	if _, err := p.Pull("x"); err == nil {
		t.Error("expected error from uninitialized Pull")
	}
	if _, err := p.List(""); err == nil {
		t.Error("expected error from uninitialized List")
	}
	if err := p.Push(nil, "x"); err == nil {
		t.Error("expected error from uninitialized Push")
	}
	if err := p.Delete("x"); err == nil {
		t.Error("expected error from uninitialized Delete")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
