package backends

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileProvider stores each secret as an encrypted file under a directory,
// suitable for committing to git.
type FileProvider struct {
	dir string
	enc Encryptor
}

// NewFileProvider returns a provider that writes to dir, creating it if needed.
func NewFileProvider(dir string, enc Encryptor) (*FileProvider, error) {
	if dir == "" {
		return nil, fmt.Errorf("file backend: directory path is required")
	}
	if enc == nil {
		return nil, fmt.Errorf("file backend: encryptor is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create remote directory: %w", err)
	}
	return &FileProvider{dir: dir, enc: enc}, nil
}

func (p *FileProvider) Push(content []byte, path string) error {
	encrypted, err := p.enc.Encrypt(content)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}

	fullPath := p.resolvePath(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	return os.WriteFile(fullPath, encrypted, 0o600)
}

func (p *FileProvider) Pull(path string) ([]byte, error) {
	encrypted, err := os.ReadFile(p.resolvePath(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return nil, fmt.Errorf("read secret file: %w", err)
	}

	plaintext, err := p.enc.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}

	return plaintext, nil
}

func (p *FileProvider) resolvePath(path string) string {
	return filepath.Join(p.dir, path+".enc")
}
