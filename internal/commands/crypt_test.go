package commands

import (
	"bytes"
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
	if _, err := os.Stat(p + ".enc"); err != nil {
		t.Error("decrypt must keep the .enc file")
	}
}

// Encryption is randomized, so a decrypt/encrypt cycle with no edit must
// keep the committed ciphertext byte for byte, or git shows a spurious diff.
func TestDecryptEncryptWithoutEditKeepsCiphertext(t *testing.T) {
	_, provider := newTestSafe(t)
	writeSecretFile(t, "db.yaml", "password: hunter2\n")
	p := filepath.Join("secrets", "db.yaml")
	if err := encryptFile(p, provider); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p + ".enc")

	if err := decryptFile(p+".enc", provider); err != nil {
		t.Fatal(err)
	}
	if err := encryptFile(p, provider); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p + ".enc")
	if !bytes.Equal(before, after) {
		t.Error("re-encrypting an unchanged secret must not rewrite the .enc file")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("encrypt must remove the plaintext even when the .enc is kept")
	}

	// An actual edit does produce a new ciphertext.
	if err := decryptFile(p+".enc", provider); err != nil {
		t.Fatal(err)
	}
	writeSecretFile(t, "db.yaml", "password: rotated\n")
	if err := encryptFile(p, provider); err != nil {
		t.Fatal(err)
	}
	edited, _ := os.ReadFile(p + ".enc")
	if bytes.Equal(before, edited) {
		t.Fatal("an edited secret must be re-encrypted")
	}
	got, err := provider.Decrypt(edited)
	if err != nil || string(got) != "password: rotated\n" {
		t.Errorf("re-encrypted content = %q, %v", got, err)
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
