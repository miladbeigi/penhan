package crypto

import (
	"bytes"
	"crypto"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type GitHubGPGProvider struct {
	entity *openpgp.Entity
}

func NewGitHubGPGProvider() *GitHubGPGProvider {
	return &GitHubGPGProvider{}
}

func (p *GitHubGPGProvider) Setup(keyPath, username string) error {
	// Try cache first — if it parses and has keys, use it
	if _, err := os.Stat(keyPath); err == nil {
		cached, err := os.ReadFile(keyPath)
		if err == nil {
			entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(cached))
			if err == nil && len(entities) > 0 {
				p.entity = entities[0]
				return nil
			}
		}
	}

	if username == "" {
		return fmt.Errorf("github username is required for github-gpg provider")
	}

	// Fetch from GitHub
	url := fmt.Sprintf("https://github.com/%s.gpg", username)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch keys from github: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github returned %d for user %q (no keys found?)", resp.StatusCode, username)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("parse github keys: %w", err)
	}
	if len(entities) == 0 {
		return fmt.Errorf("no pgp keys found for github user %q", username)
	}

	p.entity = entities[0]

	// Cache to disk
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return fmt.Errorf("create keys directory: %w", err)
	}
	if err := os.WriteFile(keyPath, body, 0o600); err != nil {
		return fmt.Errorf("cache key: %w", err)
	}

	return nil
}

func (p *GitHubGPGProvider) IsInitialized() bool {
	return p.entity != nil
}

func (p *GitHubGPGProvider) SealOnly() bool {
	return true
}

func (p *GitHubGPGProvider) Encrypt(plaintext []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return nil, err
	}

	encrypter, err := openpgp.Encrypt(w, []*openpgp.Entity{p.entity}, nil, nil, &packet.Config{DefaultHash: crypto.SHA256})
	if err != nil {
		return nil, err
	}

	if _, err := encrypter.Write(plaintext); err != nil {
		return nil, err
	}

	if err := encrypter.Close(); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (p *GitHubGPGProvider) Decrypt(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("github-gpg provider is seal-only: decryption requires the private key on your local machine")
}
