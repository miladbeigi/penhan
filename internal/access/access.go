// Package access shares a project's master key with people through their
// GitHub SSH keys.
//
// The master key encrypts each safe's key, so wrapped safe keys can be
// committed. The master key itself is encrypted with age to every pinned SSH
// public key listed in .penhan/access.yaml, and that sealed copy is committed
// as .penhan/master.age. Anyone holding one of those SSH private keys can
// unlock the master key on their machine; it is cached in the gitignored
// .penhan/master.key.
package access

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"filippo.io/age/armor"
	"github.com/miladbeigi/penhan/internal/crypto"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// Files in the project's .penhan directory.
const (
	Dir        = ".penhan"
	AccessFile = "access.yaml" // committed: who has access, and their pinned keys
	SealedFile = "master.age"  // committed: the master key, encrypted to every pinned key
	MasterFile = "master.key"  // gitignored: the unlocked master key on this machine
)

// EnvMasterKey holds a base64 master key, for CI where no SSH key is around.
const EnvMasterKey = "PENHAN_MASTER_KEY"

// ErrLocked is returned when the project has a master key but it has not been
// unlocked on this machine.
var ErrLocked = errors.New("master key is locked")

// Key is one pinned SSH public key.
type Key struct {
	Fingerprint string `yaml:"fingerprint"`
	Key         string `yaml:"key"` // authorized_keys line: "<type> <base64>"
}

// File is .penhan/access.yaml.
type File struct {
	// MasterCheck identifies the current master key without revealing it, so
	// a stale local copy is caught before it's used to re-seal or wrap.
	MasterCheck string           `yaml:"master_check"`
	Users       map[string][]Key `yaml:"users"`
}

// Check derives the MasterCheck value of a master key.
func Check(master []byte) string {
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte("penhan master key check v1"))
	return base64.RawStdEncoding.EncodeToString(mac.Sum(nil)[:16])
}

// VerifyMaster fails when master is not the key f was sealed with.
func (f *File) VerifyMaster(master []byte) error {
	if f.MasterCheck != Check(master) {
		return errors.New("your master key doesn't match this project's (it was probably rotated); run `penhan unlock` again")
	}
	return nil
}

// Path returns the path of name inside root's .penhan directory.
func Path(root, name string) string {
	return filepath.Join(root, Dir, name)
}

// FindRoot walks up from start to the directory holding .penhan/access.yaml.
// It returns "" when there is none.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(Path(dir, AccessFile)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// Load reads root's access.yaml.
func Load(root string) (*File, error) {
	data, err := os.ReadFile(Path(root, AccessFile))
	if err != nil {
		return nil, err
	}
	f := &File{}
	if err := yaml.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", AccessFile, err)
	}
	if f.Users == nil {
		f.Users = map[string][]Key{}
	}
	return f, nil
}

// Save writes root's access.yaml.
func Save(root string, f *File) error {
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o700); err != nil {
		return err
	}
	header := "# Managed by `penhan access`. Each key below can unlock master.age.\n"
	return os.WriteFile(Path(root, AccessFile), append([]byte(header), data...), 0o644)
}

// ParseKey parses an authorized_keys line into a Key. It fails for key types
// age cannot encrypt to.
func ParseKey(line string) (Key, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return Key{}, err
	}
	if _, err := agessh.ParseRecipient(line); err != nil {
		return Key{}, fmt.Errorf("%s keys are not supported (use ssh-ed25519 or ssh-rsa)", pk.Type())
	}
	// Drop the comment: it's whatever GitHub or the user put there, and
	// only the key itself matters.
	return Key{
		Fingerprint: ssh.FingerprintSHA256(pk),
		Key:         strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk))),
	}, nil
}

// Seal encrypts master to every key in f, ASCII-armored so the committed file
// diffs as text.
func Seal(master []byte, f *File) ([]byte, error) {
	var recipients []age.Recipient
	for _, user := range f.SortedUsers() {
		for _, k := range f.Users[user] {
			r, err := agessh.ParseRecipient(k.Key)
			if err != nil {
				return nil, fmt.Errorf("%s key %s: %w", user, k.Fingerprint, err)
			}
			recipients = append(recipients, r)
		}
	}
	if len(recipients) == 0 {
		return nil, errors.New("no keys to encrypt the master key to")
	}
	var buf bytes.Buffer
	aw := armor.NewWriter(&buf)
	w, err := age.Encrypt(aw, recipients...)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(master); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := aw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unseal decrypts a sealed master key with the first identity that matches.
func Unseal(sealed []byte, identities []age.Identity) ([]byte, error) {
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(sealed)), identities...)
	if err != nil {
		var noMatch *age.NoIdentityMatchError
		if errors.As(err, &noMatch) {
			return nil, errors.New("none of your SSH keys can unlock the master key; ask someone with access to run `penhan access grant <your-github-user>`, or pass --identity")
		}
		return nil, err
	}
	master, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(master) != crypto.MasterKeySize {
		return nil, fmt.Errorf("sealed master key has %d bytes, want %d", len(master), crypto.MasterKeySize)
	}
	return master, nil
}

// SortedUsers returns the user names in f, sorted.
func (f *File) SortedUsers() []string {
	users := make([]string, 0, len(f.Users))
	for u := range f.Users {
		users = append(users, u)
	}
	sort.Strings(users)
	return users
}

// WriteMaster stores the unlocked master key in root, readable only by the
// owner.
func WriteMaster(root string, master []byte) error {
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o700); err != nil {
		return err
	}
	return os.WriteFile(Path(root, MasterFile), []byte(base64.StdEncoding.EncodeToString(master)+"\n"), 0o600)
}

// LoadMaster returns the master key for the project containing start: from
// $PENHAN_MASTER_KEY, or from the unlocked copy in the project root. It
// returns a nil key and no error when the project has no master key at all,
// and fails when the key isn't the project's current one.
func LoadMaster(start string) ([]byte, error) {
	root, err := FindRoot(start)
	if err != nil || root == "" {
		return nil, err
	}
	master, err := readMaster(root)
	if err != nil {
		return nil, err
	}
	f, err := Load(root)
	if err != nil {
		return nil, err
	}
	if err := f.VerifyMaster(master); err != nil {
		return nil, err
	}
	return master, nil
}

func readMaster(root string) ([]byte, error) {
	if v := os.Getenv(EnvMasterKey); v != "" {
		master, err := decodeMaster(v)
		if err != nil {
			return nil, fmt.Errorf("$%s: %w", EnvMasterKey, err)
		}
		return master, nil
	}
	data, err := os.ReadFile(Path(root, MasterFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w; run `penhan unlock` in %s", ErrLocked, root)
	}
	if err != nil {
		return nil, err
	}
	master, err := decodeMaster(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", Path(root, MasterFile), err)
	}
	return master, nil
}

func decodeMaster(s string) ([]byte, error) {
	master, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("invalid master key: %w", err)
	}
	if len(master) != crypto.MasterKeySize {
		return nil, fmt.Errorf("invalid master key: %d bytes, want %d", len(master), crypto.MasterKeySize)
	}
	return master, nil
}
