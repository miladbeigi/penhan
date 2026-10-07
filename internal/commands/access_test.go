package commands

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/access"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/prompt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
)

// machine is a fake computer: a home directory with one SSH key pair.
type machine struct {
	home string
	key  access.Key
}

func newMachine(t *testing.T) machine {
	t.Helper()
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := ssh.NewSignerFromKey(priv)
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519"), pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_ed25519.pub"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := access.ParseKey(line)
	if err != nil {
		t.Fatal(err)
	}
	return machine{home: home, key: k}
}

// use switches HOME to m for the rest of the test (or the next use).
func (m machine) use(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", m.home)
}

// fakeGitHub serves users' keys in place of github.com.
func fakeGitHub(t *testing.T, users map[string][]access.Key) {
	t.Helper()
	orig := fetchKeys
	fetchKeys = func(_ context.Context, user string) ([]access.Key, []string, error) {
		keys, ok := users[user]
		if !ok {
			return nil, nil, os.ErrNotExist
		}
		return keys, nil, nil
	}
	t.Cleanup(func() { fetchKeys = orig })
}

// penhan runs a command in dir with fresh flag values.
func penhan(t *testing.T, dir string, args ...string) error {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	resetFlags(rootCmd)
	rootCmd.SetArgs(args)
	return rootCmd.ExecuteContext(context.Background())
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}

// addFileSafe creates a safe in root with the file backend.
func addFileSafe(t *testing.T, root, name string) string {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	if err := createSafe(&prompt.InitAnswers{SafeName: name, Encryption: "aes", Backend: "file"}); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, name)
}

// loadSafeKey loads the safe in dir like check and push do.
func loadSafeKey(t *testing.T, dir string) (crypto.Provider, error) {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()
	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return loadCryptoProvider(cfg)
}

func TestMasterKeyAcrossMachines(t *testing.T) {
	t.Setenv(access.EnvMasterKey, "")
	laptop, desktop := newMachine(t), newMachine(t)
	fakeGitHub(t, map[string][]access.Key{"milad": {laptop.key, desktop.key}})
	root := t.TempDir()

	// Laptop: an existing safe, then a master key, then wrap.
	laptop.use(t)
	safe := addFileSafe(t, root, "app")
	if err := penhan(t, root, "access", "grant", "milad"); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{access.AccessFile, access.SealedFile, access.MasterFile} {
		if _, err := os.Stat(access.Path(root, f)); err != nil {
			t.Fatalf("grant should create %s: %v", f, err)
		}
	}
	gitignore, _ := os.ReadFile(filepath.Join(root, ".gitignore"))
	if !strings.Contains(string(gitignore), ".penhan/master.key\n") {
		t.Errorf(".gitignore should ignore the master key:\n%s", gitignore)
	}
	if err := penhan(t, root, "wrap"); err != nil {
		t.Fatal(err)
	}
	laptopProvider, err := loadSafeKey(t, safe)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, _ := laptopProvider.Encrypt([]byte("password: hunter2"))

	// Desktop: a fresh clone has only committed files.
	clone := t.TempDir()
	cloneSafe := filepath.Join(clone, "app")
	copyFiles(t, root, clone, ".penhan/access.yaml", ".penhan/master.age", "app/penhan.yaml", "app/.penhan/aes.key.enc")
	desktop.use(t)
	if _, err := loadSafeKey(t, cloneSafe); err == nil || !strings.Contains(err.Error(), "penhan unlock") {
		t.Fatalf("before unlock, expected a hint to unlock, got %v", err)
	}
	if err := penhan(t, cloneSafe, "unlock"); err != nil {
		t.Fatal(err)
	}
	p, err := loadSafeKey(t, cloneSafe)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := p.Decrypt(ciphertext); err != nil || string(got) != "password: hunter2" {
		t.Fatalf("desktop decrypt = %q, %v", got, err)
	}

	// A safe added on the desktop is wrapped right away.
	other := addFileSafe(t, clone, "other")
	if _, err := os.Stat(filepath.Join(other, ".penhan", "aes.key.enc")); err != nil {
		t.Fatalf("add should wrap the new key: %v", err)
	}
}

func TestAddRefusesWhenMasterLocked(t *testing.T) {
	t.Setenv(access.EnvMasterKey, "")
	m := newMachine(t)
	m.use(t)
	fakeGitHub(t, map[string][]access.Key{"milad": {m.key}})
	root := t.TempDir()
	if err := penhan(t, root, "access", "grant", "milad"); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(access.Path(root, access.MasterFile))

	orig, _ := os.Getwd()
	_ = os.Chdir(root)
	defer func() { _ = os.Chdir(orig) }()
	err := createSafe(&prompt.InitAnswers{SafeName: "app", Encryption: "aes", Backend: "file"})
	if err == nil || !strings.Contains(err.Error(), "penhan unlock") {
		t.Fatalf("expected add to ask for unlock, got %v", err)
	}
	if _, err := os.Stat("app"); !os.IsNotExist(err) {
		t.Error("add must not leave a half-created safe")
	}
}

func TestGrantKeyChangesNeedConfirmation(t *testing.T) {
	t.Setenv(access.EnvMasterKey, "")
	a, b := newMachine(t), newMachine(t)
	a.use(t)
	users := map[string][]access.Key{"milad": {a.key}}
	fakeGitHub(t, users)
	root := t.TempDir()
	if err := penhan(t, root, "access", "grant", "milad"); err != nil {
		t.Fatal(err)
	}
	if err := penhan(t, root, "access", "grant", "milad"); err != nil {
		t.Fatalf("re-granting unchanged keys should be a no-op: %v", err)
	}

	users["milad"] = []access.Key{a.key, b.key}
	if err := penhan(t, root, "access", "grant", "milad"); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("a changed key set must be confirmed, got %v", err)
	}
	f, _ := access.Load(root)
	if len(f.Users["milad"]) != 1 {
		t.Fatal("an unconfirmed grant must not change the pinned keys")
	}
	if err := penhan(t, root, "access", "grant", "milad", "--yes"); err != nil {
		t.Fatal(err)
	}
	f, _ = access.Load(root)
	if len(f.Users["milad"]) != 2 {
		t.Fatalf("expected 2 pinned keys, got %d", len(f.Users["milad"]))
	}

	// --key limits the grant to the chosen key.
	users["bob"] = []access.Key{a.key, b.key}
	if err := penhan(t, root, "access", "grant", "bob", "--key", b.key.Fingerprint); err != nil {
		t.Fatal(err)
	}
	f, _ = access.Load(root)
	if len(f.Users["bob"]) != 1 || f.Users["bob"][0].Fingerprint != b.key.Fingerprint {
		t.Fatalf("--key should pin only %s, got %+v", b.key.Fingerprint, f.Users["bob"])
	}
}

func TestRevokeRotatesMasterKey(t *testing.T) {
	t.Setenv(access.EnvMasterKey, "")
	milad, bob := newMachine(t), newMachine(t)
	fakeGitHub(t, map[string][]access.Key{"milad": {milad.key}, "bob": {bob.key}})
	root := t.TempDir()
	milad.use(t)
	if err := penhan(t, root, "access", "grant", "milad"); err != nil {
		t.Fatal(err)
	}
	if err := penhan(t, root, "access", "grant", "bob"); err != nil {
		t.Fatal(err)
	}
	safe := addFileSafe(t, root, "app")
	p, _ := loadSafeKey(t, safe)
	ciphertext, _ := p.Encrypt([]byte("x"))
	oldMaster, _ := access.LoadMaster(root)

	if err := penhan(t, root, "access", "revoke", "bob"); err != nil {
		t.Fatal(err)
	}

	// Bob's SSH key no longer unlocks the new master.age...
	sealed, _ := os.ReadFile(access.Path(root, access.SealedFile))
	ids, _ := access.LocalIdentities([]string{filepath.Join(bob.home, ".ssh", "id_ed25519")}, nil)
	if _, err := access.Unseal(sealed, ids); err == nil {
		t.Error("the revoked user's key must not unlock the new master key")
	}
	// ...and the master key he may have kept doesn't unwrap the safe key.
	wrapped, _ := os.ReadFile(filepath.Join(safe, ".penhan", "aes.key.enc"))
	if _, err := crypto.UnwrapKey(oldMaster, wrapped); err == nil {
		t.Error("the old master key must not unwrap re-wrapped safe keys")
	}
	t.Setenv(access.EnvMasterKey, base64.StdEncoding.EncodeToString(oldMaster))
	if _, err := access.LoadMaster(root); err == nil {
		t.Error("an old master key must be rejected as stale")
	}
	t.Setenv(access.EnvMasterKey, "")

	// Milad still has everything: the safe key itself didn't change.
	_ = os.Remove(filepath.Join(safe, ".penhan", "keys", "aes.key"))
	p, err := loadSafeKey(t, safe)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := p.Decrypt(ciphertext); err != nil || string(got) != "x" {
		t.Fatalf("decrypt after revoke = %q, %v", got, err)
	}
	if err := penhan(t, root, "access", "revoke", "milad"); err == nil || !strings.Contains(err.Error(), "only user") {
		t.Fatalf("revoking the last user must fail, got %v", err)
	}
}

func copyFiles(t *testing.T, from, to string, rels ...string) {
	t.Helper()
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(from, rel))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(to, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
