package access

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"golang.org/x/crypto/ssh"
)

// PassphraseFunc asks for the passphrase of the private key at path.
type PassphraseFunc func(path string) ([]byte, error)

// LocalIdentities returns an identity for each SSH key pair in paths, or in
// ~/.ssh when paths is empty. A path may name the private key or its .pub.
//
// Each identity only reads its private key, and asks for a passphrase, when
// the sealed file was actually encrypted to its public key, so keys that
// can't unlock it are never touched.
func LocalIdentities(paths []string, passphrase PassphraseFunc) ([]age.Identity, error) {
	explicit := len(paths) > 0
	if !explicit {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		pubs, err := filepath.Glob(filepath.Join(home, ".ssh", "*.pub"))
		if err != nil {
			return nil, err
		}
		paths = pubs
	}

	var ids []age.Identity
	for _, p := range paths {
		priv := strings.TrimSuffix(p, ".pub")
		pubData, err := os.ReadFile(priv + ".pub")
		if err != nil {
			if explicit {
				return nil, fmt.Errorf("read public key for %s: %w", priv, err)
			}
			continue
		}
		pk, _, _, _, err := ssh.ParseAuthorizedKey(pubData)
		if err != nil {
			if explicit {
				return nil, fmt.Errorf("parse %s.pub: %w", priv, err)
			}
			continue
		}
		if _, err := os.Stat(priv); err != nil {
			if explicit {
				return nil, fmt.Errorf("private key %s: %w", priv, err)
			}
			continue
		}
		ids = append(ids, &lazyIdentity{path: priv, pub: pk, passphrase: passphrase})
	}
	if len(ids) == 0 {
		return nil, errors.New("no SSH key pairs found in ~/.ssh; pass --identity <private key>")
	}
	return ids, nil
}

// lazyIdentity is an SSH identity that loads its private key on first use,
// and only for a file encrypted to its public key.
type lazyIdentity struct {
	path       string
	pub        ssh.PublicKey
	passphrase PassphraseFunc
	loaded     age.Identity
}

func (i *lazyIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	if i.loaded == nil {
		if !i.matches(stanzas) {
			return nil, age.ErrIncorrectIdentity
		}
		id, err := i.load()
		if err != nil {
			return nil, err
		}
		i.loaded = id
	}
	return i.loaded.Unwrap(stanzas)
}

// matches reports whether a stanza was written for this public key. age tags
// SSH stanzas with the first 4 bytes of the key's SHA-256.
func (i *lazyIdentity) matches(stanzas []*age.Stanza) bool {
	sum := sha256.Sum256(i.pub.Marshal())
	tag := base64.RawStdEncoding.EncodeToString(sum[:4])
	for _, s := range stanzas {
		if s.Type == i.pub.Type() && len(s.Args) > 0 && s.Args[0] == tag {
			return true
		}
	}
	return false
}

func (i *lazyIdentity) load() (age.Identity, error) {
	pem, err := os.ReadFile(i.path)
	if err != nil {
		return nil, err
	}
	id, err := agessh.ParseIdentity(pem)
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		if i.passphrase == nil {
			return nil, fmt.Errorf("%s needs a passphrase, but there is no terminal to ask for it", i.path)
		}
		return agessh.NewEncryptedSSHIdentity(i.pub, pem, func() ([]byte, error) {
			return i.passphrase(i.path)
		})
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i.path, err)
	}
	return id, nil
}
