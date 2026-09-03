package commands

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/secrets"
)

// newCryptoProvider builds and initializes the encryption provider from config.
func newCryptoProvider(cfg *config.Config) (crypto.Provider, error) {
	switch cfg.Encryption.Method {
	case "gpg":
		provider := crypto.NewGPGProvider()
		if err := provider.Setup(cfg.Encryption.GPG.KeyPath, ""); err != nil {
			return nil, err
		}
		return provider, nil
	case "github-gpg":
		provider := crypto.NewGitHubGPGProvider()
		if err := provider.Setup(cfg.Encryption.GPG.KeyPath, cfg.Encryption.GPG.GitHubUsername); err != nil {
			return nil, err
		}
		return provider, nil
	case "aes":
		provider := crypto.NewAESProvider()
		if err := provider.Setup(cfg.Encryption.AES.KeyPath, ""); err != nil {
			return nil, err
		}
		return provider, nil
	default:
		return nil, fmt.Errorf("unsupported encryption method: %s", cfg.Encryption.Method)
	}
}

// newBackend builds and initializes the configured backend provider.
func newBackend(cfg *config.Config, provider crypto.Provider) (backends.Provider, error) {
	switch cfg.Backend.Type {
	case "", "vault":
		return newVaultBackend(cfg)
	case "file":
		return newFileBackend(cfg, provider)
	default:
		return nil, fmt.Errorf("unsupported backend type: %s", cfg.Backend.Type)
	}
}

func newVaultBackend(cfg *config.Config) (*backends.VaultProvider, error) {
	backend := backends.NewVaultProvider()
	token, err := os.ReadFile(cfg.Backend.Vault.TokenPath)
	if err != nil {
		return nil, err
	}
	if err := backend.Setup(backends.SetupOptions{
		Addr:      cfg.Backend.Vault.Addr,
		Token:     strings.TrimSpace(string(token)),
		MountPath: cfg.Backend.Vault.MountPath,
		BasePath:  cfg.Backend.Vault.BasePath,
	}); err != nil {
		return nil, err
	}
	return backend, nil
}

func newFileBackend(cfg *config.Config, provider crypto.Provider) (*backends.FileProvider, error) {
	dir := cfg.Backend.File.Path
	if dir == "" {
		dir = ".penhan/remote"
	}

	backend := backends.NewFileProvider()
	if err := backend.Setup(backends.SetupOptions{Dir: dir, Enc: provider}); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create remote directory: %w", err)
	}

	return backend, nil
}

// localSecret is one secret file found under the secrets directory, with its
// content canonicalized to JSON so hashes are stable across YAML/JSON and
// key order.
type localSecret struct {
	Path    string // backend path, e.g. "db/password"
	Content []byte // canonical JSON
	Hash    string
}

// collectLocalSecrets walks the secrets directory and returns each secret,
// sorted by path. Encrypted files are decrypted with provider; when both
// plaintext and .enc exist for the same secret, the plaintext wins (it is
// the editable copy).
func collectLocalSecrets(cfg *config.Config, provider crypto.Provider) ([]localSecret, error) {
	byPath := make(map[string]localSecret)

	err := filepath.Walk(cfg.Secrets.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		name := path
		isEnc := strings.HasSuffix(name, ".enc")
		if isEnc {
			name = strings.TrimSuffix(name, ".enc")
		}
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil
		}

		remotePath := secrets.LocalToVault(name, cfg.Secrets.Path)
		if _, seen := byPath[remotePath]; seen && isEnc {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isEnc {
			if data, err = provider.Decrypt(data); err != nil {
				return fmt.Errorf("decrypt %s: %w", path, err)
			}
		}

		parsed, err := secrets.Parse(data, ext)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		content, err := json.Marshal(parsed)
		if err != nil {
			return err
		}

		byPath[remotePath] = localSecret{Path: remotePath, Content: content, Hash: hashContent(content)}
		return nil
	})
	if err != nil {
		return nil, err
	}

	list := make([]localSecret, 0, len(byPath))
	for _, s := range byPath {
		list = append(list, s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
	return list, nil
}

// remoteHash returns the hash of the secret stored at path, or "" when the
// backend has nothing there. Any other backend failure is returned as is.
func remoteHash(backend backends.Provider, path string) (string, error) {
	content, err := backend.Pull(path)
	if err != nil {
		if errors.Is(err, backends.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("read %s from backend: %w", path, err)
	}
	return hashContent(canonicalJSON(content)), nil
}

// canonicalJSON re-encodes JSON so key order and whitespace cannot make two
// equal documents hash differently. Content that is not a JSON object is
// returned unchanged.
func canonicalJSON(content []byte) []byte {
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return content
	}
	out, err := json.Marshal(data)
	if err != nil {
		return content
	}
	return out
}

func hashContent(content []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(content))
}
