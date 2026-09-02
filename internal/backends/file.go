package backends

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileProvider struct {
	dir string
	enc Encryptor
}

func NewFileProvider() *FileProvider {
	return &FileProvider{}
}

func (p *FileProvider) Setup(opts SetupOptions) error {
	if opts.Dir == "" {
		return fmt.Errorf("file provider: directory path is required")
	}
	if opts.Enc == nil {
		return fmt.Errorf("file provider: encryptor is required")
	}
	p.dir = opts.Dir
	p.enc = opts.Enc
	return nil
}

func (p *FileProvider) IsInitialized() bool {
	return p.dir != "" && p.enc != nil
}

func (p *FileProvider) Push(content []byte, remotePath string) error {
	if !p.IsInitialized() {
		return fmt.Errorf("file provider not initialized")
	}

	encrypted, err := p.enc.Encrypt(content)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}

	fullPath := p.resolvePath(remotePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	return os.WriteFile(fullPath, encrypted, 0o600)
}

func (p *FileProvider) Pull(remotePath string) ([]byte, error) {
	if !p.IsInitialized() {
		return nil, fmt.Errorf("file provider not initialized")
	}

	fullPath := p.resolvePath(remotePath)
	encrypted, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("secret not found: %s", remotePath)
		}
		return nil, fmt.Errorf("read secret file: %w", err)
	}

	plaintext, err := p.enc.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}

	return plaintext, nil
}

func (p *FileProvider) List(remotePath string) ([]string, error) {
	if !p.IsInitialized() {
		return nil, fmt.Errorf("file provider not initialized")
	}

	searchDir := p.dir
	if remotePath != "" {
		searchDir = filepath.Join(p.dir, remotePath)
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list directory: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			subDir := remotePath
			if subDir != "" {
				subDir = remotePath + "/" + name
			} else {
				subDir = name
			}
			sub, err := p.List(subDir)
			if err != nil {
				return nil, err
			}
			paths = append(paths, sub...)
			continue
		}

		if !strings.HasSuffix(name, ".enc") {
			continue
		}

		rel := strings.TrimSuffix(name, ".enc")
		if remotePath != "" {
			rel = remotePath + "/" + rel
		}
		paths = append(paths, rel)
	}

	return paths, nil
}

func (p *FileProvider) Delete(remotePath string) error {
	if !p.IsInitialized() {
		return fmt.Errorf("file provider not initialized")
	}

	fullPath := p.resolvePath(remotePath)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete secret file: %w", err)
	}

	return nil
}

func (p *FileProvider) resolvePath(remotePath string) string {
	return filepath.Join(p.dir, remotePath+".enc")
}
