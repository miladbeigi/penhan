//go:build e2e

package e2e

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// newHome creates a home directory with an SSH key pair and returns it and
// the public key line.
func newHome(t *testing.T) (home, pub string) {
	t.Helper()
	home = t.TempDir()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := ssh.NewSignerFromKey(priv)
	pub = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	writeFile(t, home, ".ssh/id_ed25519", string(pem.EncodeToMemory(block)))
	writeFile(t, home, ".ssh/id_ed25519.pub", pub+"\n")
	return home, pub
}

// TestMasterKeyJourney: a safe created on one machine is pushed from a fresh
// clone on another machine, which only gets the committed files and unlocks
// the master key with its own SSH key. CI does the same with
// PENHAN_MASTER_KEY.
func TestMasterKeyJourney(t *testing.T) {
	vault := startVault(t)
	laptop, laptopPub := newHome(t)
	desktop, desktopPub := newHome(t)
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/milad.keys" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(laptopPub + "\n" + desktopPub + "\n"))
	}))
	defer github.Close()
	envFor := func(home string) []string {
		return []string{"HOME=" + home, "PENHAN_GITHUB_URL=" + github.URL, "PENHAN_MASTER_KEY="}
	}

	// Laptop: a safe exists before the master key does.
	repo := newProject(t)
	safe := addSafe(t, repo, "app", vault)
	writeFile(t, safe, "secrets/db.yaml", "password: hunter2\n")
	stdout, stderr, code := runEnv(t, safe, envFor(laptop), "encrypt")
	requireSuccess(t, "encrypt", stdout, stderr, code)
	stdout, stderr, code = runEnv(t, repo, envFor(laptop), "access", "grant", "milad")
	requireSuccess(t, "access grant", stdout, stderr, code)
	if !strings.Contains(stdout, "2 key(s)") {
		t.Errorf("grant should pin both keys:\n%s", stdout)
	}
	stdout, stderr, code = runEnv(t, repo, envFor(laptop), "wrap")
	requireSuccess(t, "wrap", stdout, stderr, code)

	// Only committed files reach the clone.
	clone := newProject(t)
	for _, rel := range []string{
		".penhan/access.yaml", ".penhan/master.age",
		"app/penhan.yaml", "app/.penhan/aes.key.enc", "app/secrets/db.yaml.enc",
	} {
		writeFile(t, clone, rel, readFile(t, repo, rel))
	}
	writeFile(t, clone, "app/.penhan/vault-token", vault.token)
	cloneSafe := filepath.Join(clone, "app")

	_, stderr, code = runEnv(t, cloneSafe, envFor(desktop), "push")
	if code == 0 || !strings.Contains(stderr, "penhan unlock") {
		t.Fatalf("push before unlock should ask to unlock, code %d:\n%s", code, stderr)
	}
	stdout, stderr, code = runEnv(t, cloneSafe, envFor(desktop), "unlock")
	requireSuccess(t, "unlock", stdout, stderr, code)
	stdout, stderr, code = runEnv(t, cloneSafe, envFor(desktop), "push")
	requireSuccess(t, "push from clone", stdout, stderr, code)
	if data := vaultData(t, vault, "secret/data/app/db"); data == nil || data["password"] != "hunter2" {
		t.Fatalf("vault should hold the secret pushed from the clone, got %v", data)
	}

	// CI: no SSH key, the master key comes from the environment.
	master := strings.TrimSpace(readFile(t, clone, ".penhan/master.key"))
	ci := newProject(t)
	for _, rel := range []string{".penhan/access.yaml", "app/penhan.yaml", "app/.penhan/aes.key.enc", "app/secrets/db.yaml.enc"} {
		writeFile(t, ci, rel, readFile(t, repo, rel))
	}
	writeFile(t, ci, "app/.penhan/vault-token", vault.token)
	stdout, stderr, code = runEnv(t, filepath.Join(ci, "app"),
		[]string{"HOME=" + t.TempDir(), "PENHAN_MASTER_KEY=" + master}, "check")
	requireSuccess(t, "check in CI", stdout, stderr, code)

	if _, err := os.Stat(filepath.Join(cloneSafe, ".penhan", "keys", "aes.key")); !os.IsNotExist(err) {
		t.Error("the clone should never have needed a plaintext safe key")
	}
}
