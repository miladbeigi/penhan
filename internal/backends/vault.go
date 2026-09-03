package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

type VaultProvider struct {
	client    *vault.Client
	addr      string
	token     string
	mountPath string
	basePath  string
}

func NewVaultProvider() *VaultProvider {
	return &VaultProvider{}
}

func (p *VaultProvider) Setup(opts SetupOptions) error {
	p.addr = opts.Addr
	p.token = opts.Token
	p.mountPath = opts.MountPath
	p.basePath = opts.BasePath

	config := vault.DefaultConfig()
	config.Address = opts.Addr

	client, err := vault.NewClient(config)
	if err != nil {
		return fmt.Errorf("create vault client: %w", err)
	}

	client.SetToken(opts.Token)
	p.client = client

	return nil
}

func (p *VaultProvider) IsInitialized() bool {
	return p.client != nil
}

func (p *VaultProvider) Push(content []byte, remotePath string) error {
	if !p.IsInitialized() {
		return fmt.Errorf("vault provider not initialized")
	}

	fullPath := p.buildPath(remotePath)

	data := make(map[string]interface{})
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("unmarshal secret data: %w", err)
	}

	secretData := map[string]interface{}{
		"data": data,
	}

	_, err := p.client.Logical().WriteWithContext(context.TODO(), fullPath, secretData)
	if err != nil {
		return fmt.Errorf("write to vault: %w", err)
	}

	return nil
}

func (p *VaultProvider) Pull(remotePath string) ([]byte, error) {
	if !p.IsInitialized() {
		return nil, fmt.Errorf("vault provider not initialized")
	}

	fullPath := p.buildPath(remotePath)

	secret, err := p.client.Logical().ReadWithContext(context.TODO(), fullPath)
	if err != nil {
		return nil, fmt.Errorf("read from vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, remotePath)
	}

	// A soft-deleted KV v2 version still answers the read, but with nil data.
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok || data == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, remotePath)
	}

	content, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal secret data: %w", err)
	}

	return content, nil
}

// List returns every secret under remotePath, recursing into KV v2 folders so
// nested paths like "apps/api-token" are returned in full, never as bare
// folder names.
func (p *VaultProvider) List(remotePath string) ([]string, error) {
	if !p.IsInitialized() {
		return nil, fmt.Errorf("vault provider not initialized")
	}

	// KV v2 only supports LIST on the metadata path, not the data path.
	fullPath := p.buildPathWithPrefix("metadata", remotePath)
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}

	secret, err := p.client.Logical().ListWithContext(context.TODO(), fullPath)
	if err != nil {
		return nil, fmt.Errorf("list vault secrets: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return []string{}, nil
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return []string{}, nil
	}

	paths := []string{}
	for _, key := range keys {
		k, ok := key.(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(k, "/") {
			sub, err := p.List(path.Join(remotePath, strings.TrimSuffix(k, "/")))
			if err != nil {
				return nil, err
			}
			paths = append(paths, sub...)
			continue
		}
		paths = append(paths, path.Join(remotePath, k))
	}

	return paths, nil
}

func (p *VaultProvider) Delete(remotePath string) error {
	if !p.IsInitialized() {
		return fmt.Errorf("vault provider not initialized")
	}

	fullPath := p.buildPath(remotePath)

	_, err := p.client.Logical().DeleteWithContext(context.TODO(), fullPath)
	if err != nil {
		return fmt.Errorf("delete from vault: %w", err)
	}

	return nil
}

func (p *VaultProvider) buildPath(remotePath string) string {
	return p.buildPathWithPrefix("data", remotePath)
}

func (p *VaultProvider) buildPathWithPrefix(prefix, remotePath string) string {
	parts := []string{p.mountPath, prefix}
	if p.basePath != "" {
		parts = append(parts, p.basePath)
	}
	parts = append(parts, remotePath)
	return strings.Join(parts, "/")
}
