package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

// VaultOptions configures a VaultProvider.
type VaultOptions struct {
	Addr      string
	Token     string
	MountPath string // KV v2 mount, e.g. "secret"
	BasePath  string // prefix under the mount, normally the safe name
}

// VaultProvider stores secrets in a HashiCorp Vault KV v2 mount.
type VaultProvider struct {
	client    *vault.Client
	mountPath string
	basePath  string
}

func NewVaultProvider(opts VaultOptions) (*VaultProvider, error) {
	config := vault.DefaultConfig()
	config.Address = opts.Addr

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}
	client.SetToken(opts.Token)

	return &VaultProvider{client: client, mountPath: opts.MountPath, basePath: opts.BasePath}, nil
}

func (p *VaultProvider) Push(content []byte, remotePath string) error {
	fullPath := p.buildPath(remotePath)

	var data map[string]interface{}
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

// buildPath returns the KV v2 data path: {mount}/data/{base}/{path}.
func (p *VaultProvider) buildPath(remotePath string) string {
	parts := []string{p.mountPath, "data"}
	if p.basePath != "" {
		parts = append(parts, p.basePath)
	}
	parts = append(parts, remotePath)
	return strings.Join(parts, "/")
}
