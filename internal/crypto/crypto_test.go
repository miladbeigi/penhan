package crypto

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generate(t *testing.T, method string) string {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), method+".key")
	if err := Generate(method, keyPath); err != nil {
		t.Fatalf("Generate(%s) error = %v", method, err)
	}
	return keyPath
}

func load(t *testing.T, method, keyPath string) Provider {
	t.Helper()
	p, err := Load(method, keyPath)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", method, err)
	}
	return p
}

func TestRoundtrip(t *testing.T) {
	plaintexts := []string{"", "secret", "こんにちは世界", "!@#$%^&*()_+-=[]{}|;':\",./<>?", strings.Repeat("x", 1<<16)}
	for _, method := range []string{MethodGPG, MethodAES} {
		t.Run(method, func(t *testing.T) {
			p := load(t, method, generate(t, method))
			for _, want := range plaintexts {
				ct, err := p.Encrypt([]byte(want))
				if err != nil {
					t.Fatalf("Encrypt() error = %v", err)
				}
				if want != "" && strings.Contains(string(ct), want) {
					t.Fatal("ciphertext contains the plaintext")
				}
				got, err := p.Decrypt(ct)
				if err != nil {
					t.Fatalf("Decrypt() error = %v", err)
				}
				if string(got) != want {
					t.Errorf("Decrypt() = %q, want %q", got, want)
				}
			}
		})
	}
}

// A key loaded in a later invocation must decrypt what an earlier one encrypted.
func TestKeyPersistsAcrossLoads(t *testing.T) {
	for _, method := range []string{MethodGPG, MethodAES} {
		t.Run(method, func(t *testing.T) {
			keyPath := generate(t, method)
			ct, err := load(t, method, keyPath).Encrypt([]byte("persist"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := load(t, method, keyPath).Decrypt(ct)
			if err != nil || string(got) != "persist" {
				t.Fatalf("Decrypt() = %q, %v", got, err)
			}
		})
	}
}

// Load must never create a key: a fresh clone without the key would
// otherwise encrypt new files with a different key than the rest of the safe.
func TestLoadMissingKeyDoesNotCreateOne(t *testing.T) {
	for _, method := range []string{MethodGPG, MethodAES} {
		t.Run(method, func(t *testing.T) {
			keyPath := filepath.Join(t.TempDir(), "missing.key")
			_, err := Load(method, keyPath)
			if !errors.Is(err, ErrKeyNotFound) {
				t.Fatalf("Load() error = %v, want ErrKeyNotFound", err)
			}
			if _, statErr := os.Stat(keyPath); !os.IsNotExist(statErr) {
				t.Fatalf("Load() must not create %s", keyPath)
			}
		})
	}
}

func TestGenerateRefusesToOverwrite(t *testing.T) {
	for _, method := range []string{MethodGPG, MethodAES} {
		t.Run(method, func(t *testing.T) {
			keyPath := generate(t, method)
			before, _ := os.ReadFile(keyPath)
			if err := Generate(method, keyPath); err == nil {
				t.Fatal("Generate() over an existing key must fail")
			}
			after, _ := os.ReadFile(keyPath)
			if string(before) != string(after) {
				t.Fatal("existing key was modified")
			}
		})
	}
}

func TestGeneratedKeyIsOwnerOnly(t *testing.T) {
	for _, method := range []string{MethodGPG, MethodAES} {
		t.Run(method, func(t *testing.T) {
			info, err := os.Stat(generate(t, method))
			if err != nil {
				t.Fatal(err)
			}
			if perm := info.Mode().Perm(); perm != 0o600 {
				t.Errorf("key mode = %o, want 600", perm)
			}
		})
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	for _, method := range []string{MethodGPG, MethodAES} {
		t.Run(method, func(t *testing.T) {
			ct, err := load(t, method, generate(t, method)).Encrypt([]byte("secret"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := load(t, method, generate(t, method)).Decrypt(ct); err == nil {
				t.Fatal("Decrypt() with another safe's key must fail")
			}
		})
	}
}

func TestLoadRejectsInvalidAESKey(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "aes.key")
	if err := os.WriteFile(keyPath, []byte("too short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(MethodAES, keyPath); err == nil {
		t.Fatal("Load() must reject a key of the wrong size")
	}
}

func TestUnsupportedMethods(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "k")
	for _, tc := range []struct{ method, want string }{
		{"rot13", "unsupported"},
		{"", "unsupported"},
		{"github-gpg", "no longer supported"},
	} {
		if err := Generate(tc.method, keyPath); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Generate(%q) error = %v, want %q", tc.method, err, tc.want)
		}
		if _, err := Load(tc.method, keyPath); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Load(%q) error = %v, want %q", tc.method, err, tc.want)
		}
	}
}
