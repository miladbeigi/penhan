package backends

import (
	"context"
	"encoding/json"
	"fmt"
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

func (p *VaultProvider) Setup(addr, token, mountPath, basePath string) error {
	p.addr = addr
	p.token = token
	p.mountPath = mountPath
	p.basePath = basePath

	config := vault.DefaultConfig()
	config.Address = addr

	client, err := vault.NewClient(config)
	if err != nil {
		return fmt.Errorf("create vault client: %w", err)
	}

	client.SetToken(token)
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
		return nil, fmt.Errorf("secret not found: %s", remotePath)
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid secret format")
	}

	content, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal secret data: %w", err)
	}

	return content, nil
}

func (p *VaultProvider) List(remotePath string) ([]string, error) {
	if !p.IsInitialized() {
		return nil, fmt.Errorf("vault provider not initialized")
	}

	fullPath := p.buildPath(remotePath)
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

	var paths []string
	for _, key := range keys {
		if k, ok := key.(string); ok {
			paths = append(paths, strings.TrimSuffix(k, "/"))
		}
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
	parts := []string{p.mountPath, "data"}
	if p.basePath != "" {
		parts = append(parts, p.basePath)
	}
	parts = append(parts, remotePath)
	return strings.Join(parts, "/")
}
