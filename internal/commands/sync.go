package commands

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/secrets"
	"github.com/miladbeigi/penhan/internal/state"
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

// newVaultBackend builds and initializes the Vault backend from config.
func newVaultBackend(cfg *config.Config) (*backends.VaultProvider, error) {
	backend := backends.NewVaultProvider()
	token, err := os.ReadFile(cfg.Backend.Vault.TokenPath)
	if err != nil {
		return nil, err
	}
	if err := backend.Setup(cfg.Backend.Vault.Addr, strings.TrimSpace(string(token)), cfg.Backend.Vault.MountPath, cfg.Backend.Vault.BasePath); err != nil {
		return nil, err
	}
	return backend, nil
}

// collectLocalSecrets walks the secrets directory and returns each secret's
// content as canonical JSON, keyed by its Vault path. Encrypted files are
// decrypted with provider; when both plaintext and .enc exist for the same
// secret, the plaintext wins (it is the editable copy).
func collectLocalSecrets(cfg *config.Config, provider crypto.Provider) (map[string][]byte, error) {
	local := make(map[string][]byte)

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

		vaultPath := secrets.LocalToVault(name, cfg.Secrets.Path)
		if _, seen := local[vaultPath]; seen && isEnc {
			// Plaintext sibling was already collected (walk order is lexical).
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

		local[vaultPath] = content
		return nil
	})
	if err != nil {
		return nil, err
	}

	return local, nil
}

// buildLocalState derives a plan-ready state from the local secret files,
// marking entries changed when their content differs from the last sync.
func buildLocalState(prev *state.State, local map[string][]byte) *state.State {
	localState := state.NewState()
	for path, content := range local {
		hash := hashContent(content)
		status := "local_changed"
		if entry, ok := prev.Secrets[path]; ok && entry.LocalHash == hash {
			status = "synced"
		}
		localState.Secrets[path] = state.SecretEntry{LocalHash: hash, Status: status}
	}
	return localState
}

// buildRemoteState derives a plan-ready state from the backend, marking entries
// changed when their content differs from the last sync. Secrets that cannot be
// read (e.g. deleted versions) are skipped.
func buildRemoteState(prev *state.State, backend *backends.VaultProvider) (*state.State, error) {
	remoteState := state.NewState()
	paths, err := backend.List("")
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		content, err := backend.Pull(path)
		if err != nil {
			continue
		}
		hash := hashContent(content)
		status := "remote_changed"
		if entry, ok := prev.Secrets[path]; ok && entry.RemoteHash == hash {
			status = "synced"
		}
		remoteState.Secrets[path] = state.SecretEntry{LocalHash: hash, Status: status}
	}
	return remoteState, nil
}

// fetchRemoteState reads all secrets from the backend without change detection.
// Used by pull where we want to accept remote as-is.
func fetchRemoteState(backend *backends.VaultProvider) (*state.State, error) {
	remoteState := state.NewState()
	paths, err := backend.List("")
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		content, err := backend.Pull(path)
		if err != nil {
			continue
		}
		remoteState.Secrets[path] = state.SecretEntry{LocalHash: hashContent(content)}
	}
	return remoteState, nil
}

func hashContent(content []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(content))
}

// loadStateOrNew reads the state file, returning a fresh state when it does not exist.
func loadStateOrNew(statePath string) (*state.State, error) {
	s, err := state.Load(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state.NewState(), nil
		}
		return nil, fmt.Errorf("load state: %w", err)
	}
	return s, nil
}
