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

// loadSafeConfig reads penhan.yaml from the current directory and turns the
// missing-file case into a message that says how to fix it.
func loadSafeConfig() (*config.Config, error) {
	cfg, err := config.Load("penhan.yaml")
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no penhan.yaml in the current directory; run this command inside a safe (created with `penhan add`)")
	}
	return cfg, err
}

// openSafe loads the safe in the current directory and its encryption key.
func openSafe() (*config.Config, crypto.Provider, error) {
	cfg, err := loadSafeConfig()
	if err != nil {
		return nil, nil, err
	}
	provider, err := loadCryptoProvider(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cfg, provider, nil
}

// loadCryptoProvider loads the safe's encryption key. It never generates
// one: only `penhan add` creates keys.
func loadCryptoProvider(cfg *config.Config) (crypto.Provider, error) {
	return crypto.Load(cfg.Encryption.Method, cfg.Encryption.KeyPath())
}

// newBackend builds the configured backend provider.
func newBackend(cfg *config.Config, provider crypto.Provider) (backends.Provider, error) {
	switch cfg.Backend.Type {
	case "", backends.TypeVault:
		return newVaultBackend(cfg)
	case backends.TypeFile:
		dir := cfg.Backend.File.Path
		if dir == "" {
			dir = defaultRemoteDir
		}
		return backends.NewFileProvider(dir, provider)
	case backends.TypeKubernetes:
		k := cfg.Backend.Kubernetes
		return backends.NewKubernetesProvider(backends.KubernetesOptions{
			Kubeconfig: k.Kubeconfig,
			Context:    k.Context,
			Namespace:  k.Namespace,
			Safe:       k.Safe,
		})
	default:
		return nil, fmt.Errorf("unsupported backend type: %q", cfg.Backend.Type)
	}
}

func newVaultBackend(cfg *config.Config) (*backends.VaultProvider, error) {
	token, err := os.ReadFile(cfg.Backend.Vault.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("read vault token: %w", err)
	}
	return backends.NewVaultProvider(backends.VaultOptions{
		Addr:      cfg.Backend.Vault.Addr,
		Token:     strings.TrimSpace(string(token)),
		MountPath: cfg.Backend.Vault.MountPath,
		BasePath:  cfg.Backend.Vault.BasePath,
	})
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

		if !secrets.IsSecretFile(path) {
			return nil
		}
		name := strings.TrimSuffix(path, ".enc")
		isEnc := name != path
		ext := filepath.Ext(name)

		remotePath := secrets.RemotePath(name, cfg.Secrets.Path)
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
