package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	_, provider := newTestSafe(t)
	writeSecretFile(t, "db.yaml", "password: hunter2\n")
	p := filepath.Join("secrets", "db.yaml")

	if err := encryptFile(p, provider); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("encrypt must remove the plaintext")
	}
	enc, err := os.ReadFile(p + ".enc")
	if err != nil || strings.Contains(string(enc), "hunter2") {
		t.Fatalf("expected an encrypted .enc file, err=%v", err)
	}

	if err := decryptFile(p+".enc", provider); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "password: hunter2\n" {
		t.Fatalf("decrypt round-trip = %q, %v", got, err)
	}
	if _, err := os.Stat(p + ".enc"); !os.IsNotExist(err) {
		t.Error("decrypt must remove the .enc file")
	}
}

func TestDecryptWritesOwnerOnlyPlaintext(t *testing.T) {
	_, provider := newTestSafe(t)
	writeSecretFile(t, "db.yaml", "k: v\n")
	p := filepath.Join("secrets", "db.yaml")
	if err := encryptFile(p, provider); err != nil {
		t.Fatal(err)
	}
	if err := decryptFile(p+".enc", provider); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("decrypted plaintext mode = %o, want 600", perm)
	}
}

// Decrypting must never silently discard local edits to the plaintext.
func TestDecryptRefusesToClobberEditedPlaintext(t *testing.T) {
	_, provider := newTestSafe(t)
	p := filepath.Join("secrets", "db.yaml")
	enc, err := provider.Encrypt([]byte("k: old\n"))
	if err != nil {
		t.Fatal(err)
	}
	writeSecretFile(t, "db.yaml.enc", string(enc))
	writeSecretFile(t, "db.yaml", "k: edited\n")

	if err := decryptFile(p+".enc", provider); err == nil {
		t.Fatal("decrypt must refuse to overwrite a plaintext with different content")
	}
	if got, _ := os.ReadFile(p); string(got) != "k: edited\n" {
		t.Errorf("local edit was lost: %q", got)
	}
	if _, err := os.Stat(p + ".enc"); err != nil {
		t.Error("the .enc file must be kept when decrypt refuses")
	}
}

func TestDecryptAcceptsIdenticalPlaintext(t *testing.T) {
	_, provider := newTestSafe(t)
	p := filepath.Join("secrets", "db.yaml")
	enc, err := provider.Encrypt([]byte("k: v\n"))
	if err != nil {
		t.Fatal(err)
	}
	writeSecretFile(t, "db.yaml.enc", string(enc))
	writeSecretFile(t, "db.yaml", "k: v\n")

	if err := decryptFile(p+".enc", provider); err != nil {
		t.Fatalf("decrypt with an identical plaintext should succeed: %v", err)
	}
	if _, err := os.Stat(p + ".enc"); !os.IsNotExist(err) {
		t.Error("decrypt must remove the .enc file")
	}
}

func TestEncryptSkipsNonSecretFiles(t *testing.T) {
	_, provider := newTestSafe(t)
	writeSecretFile(t, ".gitkeep", "")
	p := filepath.Join("secrets", ".gitkeep")

	if err := encryptFile(p, provider); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Error("non-secret files must be left in place")
	}
	if _, err := os.Stat(p + ".enc"); !os.IsNotExist(err) {
		t.Error("non-secret files must not be encrypted")
	}
}
