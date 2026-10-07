package access

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/crypto"
	"golang.org/x/crypto/ssh"
)

// writeKeyPair writes an SSH key pair to dir/name and returns its
// authorized_keys line. A non-empty passphrase encrypts the private key.
func writeKeyPair(t *testing.T, dir, name, kind, passphrase string) string {
	t.Helper()
	var priv any
	switch kind {
	case "ed25519":
		_, priv, _ = ed25519.GenerateKey(rand.Reader)
	case "rsa":
		priv, _ = rsa.GenerateKey(rand.Reader, 2048)
	case "ecdsa":
		priv, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	var block *pem.Block
	var err error
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if err := os.WriteFile(filepath.Join(dir, name), pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".pub"), []byte(line+" test@host\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return line
}

func mustKey(t *testing.T, line string) Key {
	t.Helper()
	k, err := ParseKey(line)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealUnsealWithMultipleKeys(t *testing.T) {
	dir := t.TempDir()
	ed := writeKeyPair(t, dir, "id_ed25519", "ed25519", "")
	rsaKey := writeKeyPair(t, dir, "id_rsa", "rsa", "")
	master, _ := crypto.GenerateMasterKey()
	f := &File{Users: map[string][]Key{"alice": {mustKey(t, ed), mustKey(t, rsaKey)}}}

	sealed, err := Seal(master, f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(sealed, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		t.Error("sealed master key should be ASCII-armored")
	}
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		ids, err := LocalIdentities([]string{filepath.Join(dir, name)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Unseal(sealed, ids)
		if err != nil || !bytes.Equal(got, master) {
			t.Fatalf("unseal with %s = %v", name, err)
		}
	}
}

func TestUnsealNoMatchingKey(t *testing.T) {
	dir := t.TempDir()
	granted := writeKeyPair(t, dir, "granted", "ed25519", "")
	writeKeyPair(t, dir, "other", "ed25519", "")
	master, _ := crypto.GenerateMasterKey()
	sealed, _ := Seal(master, &File{Users: map[string][]Key{"a": {mustKey(t, granted)}}})

	ids, _ := LocalIdentities([]string{filepath.Join(dir, "other")}, nil)
	_, err := Unseal(sealed, ids)
	if err == nil || !strings.Contains(err.Error(), "penhan access grant") {
		t.Fatalf("expected a hint to get access, got %v", err)
	}
}

// Keys the file wasn't encrypted to are never read, so a broken or
// passphrase-protected unrelated key costs nothing.
func TestUnsealOnlyTouchesMatchingKey(t *testing.T) {
	dir := t.TempDir()
	granted := writeKeyPair(t, dir, "granted", "ed25519", "")
	writeKeyPair(t, dir, "locked", "ed25519", "secret")
	writeKeyPair(t, dir, "broken", "ed25519", "")
	if err := os.WriteFile(filepath.Join(dir, "broken"), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	master, _ := crypto.GenerateMasterKey()
	sealed, _ := Seal(master, &File{Users: map[string][]Key{"a": {mustKey(t, granted)}}})

	asked := 0
	ids, err := LocalIdentities([]string{
		filepath.Join(dir, "broken"), filepath.Join(dir, "locked"), filepath.Join(dir, "granted"),
	}, func(string) ([]byte, error) { asked++; return []byte("secret"), nil })
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unseal(sealed, ids)
	if err != nil || !bytes.Equal(got, master) {
		t.Fatalf("Unseal = %v", err)
	}
	if asked != 0 {
		t.Errorf("asked for %d passphrase(s) of keys that can't unlock the file", asked)
	}
}

func TestUnsealPassphraseProtectedKey(t *testing.T) {
	dir := t.TempDir()
	line := writeKeyPair(t, dir, "id_ed25519", "ed25519", "hunter2")
	master, _ := crypto.GenerateMasterKey()
	sealed, _ := Seal(master, &File{Users: map[string][]Key{"a": {mustKey(t, line)}}})

	path := filepath.Join(dir, "id_ed25519")
	ids, _ := LocalIdentities([]string{path}, nil)
	if _, err := Unseal(sealed, ids); err == nil || !strings.Contains(err.Error(), "passphrase") {
		t.Fatalf("without a terminal, expected a passphrase error, got %v", err)
	}

	ids, _ = LocalIdentities([]string{path}, func(p string) ([]byte, error) {
		if p != path {
			t.Errorf("asked for passphrase of %s", p)
		}
		return []byte("hunter2"), nil
	})
	got, err := Unseal(sealed, ids)
	if err != nil || !bytes.Equal(got, master) {
		t.Fatalf("Unseal = %v", err)
	}
}

func TestParseKeyRejectsECDSA(t *testing.T) {
	line := writeKeyPair(t, t.TempDir(), "id_ecdsa", "ecdsa", "")
	if _, err := ParseKey(line); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected ecdsa to be rejected, got %v", err)
	}
}

func TestParseKeyDropsComment(t *testing.T) {
	line := writeKeyPair(t, t.TempDir(), "k", "ed25519", "")
	k := mustKey(t, line+" laptop")
	if strings.Contains(k.Key, "laptop") || !strings.HasPrefix(k.Fingerprint, "SHA256:") {
		t.Errorf("unexpected key %+v", k)
	}
}

func TestFetchGitHubKeys(t *testing.T) {
	dir := t.TempDir()
	ed := writeKeyPair(t, dir, "a", "ed25519", "")
	ec := writeKeyPair(t, dir, "b", "ecdsa", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/alice.keys":
			_, _ = w.Write([]byte(ed + "\n" + ec + "\n"))
		case "/ecdsa-only.keys":
			_, _ = w.Write([]byte(ec + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("PENHAN_GITHUB_URL", srv.URL)

	keys, skipped, err := FetchGitHubKeys(context.Background(), "alice")
	if err != nil || len(keys) != 1 || len(skipped) != 1 {
		t.Fatalf("got %d keys, %d skipped, err %v", len(keys), len(skipped), err)
	}
	if _, _, err := FetchGitHubKeys(context.Background(), "ecdsa-only"); err == nil {
		t.Error("a user with no usable keys must fail")
	}
	if _, _, err := FetchGitHubKeys(context.Background(), "nobody"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found, got %v", err)
	}
	if _, _, err := FetchGitHubKeys(context.Background(), "../etc"); err == nil {
		t.Error("invalid usernames must be rejected")
	}
}

func TestLoadMaster(t *testing.T) {
	root := t.TempDir()
	safe := filepath.Join(root, "app")
	if err := os.MkdirAll(safe, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvMasterKey, "")

	if m, err := LoadMaster(safe); m != nil || err != nil {
		t.Fatalf("no project master key: got %v, %v", m, err)
	}

	master, _ := crypto.GenerateMasterKey()
	if err := Save(root, &File{MasterCheck: Check(master), Users: map[string][]Key{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMaster(safe); !errors.Is(err, ErrLocked) {
		t.Fatalf("expected ErrLocked, got %v", err)
	}

	if err := WriteMaster(root, master); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(Path(root, MasterFile)); info.Mode().Perm() != 0o600 {
		t.Errorf("master.key mode = %v, want 0600", info.Mode().Perm())
	}
	if m, err := LoadMaster(safe); err != nil || !bytes.Equal(m, master) {
		t.Fatalf("LoadMaster = %v", err)
	}

	stale, _ := crypto.GenerateMasterKey()
	t.Setenv(EnvMasterKey, base64.StdEncoding.EncodeToString(stale))
	if _, err := LoadMaster(safe); err == nil || !strings.Contains(err.Error(), "unlock") {
		t.Fatalf("a stale master key must be rejected, got %v", err)
	}
	t.Setenv(EnvMasterKey, base64.StdEncoding.EncodeToString(master))
	if m, err := LoadMaster(safe); err != nil || !bytes.Equal(m, master) {
		t.Fatalf("LoadMaster from env = %v", err)
	}
}
