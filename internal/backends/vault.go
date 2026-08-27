package backends

import (
	"fmt"

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

func (p *VaultProvider) Setup(addr string, token string, mountPath string, basePath string) error {
	p.addr = addr
	p.token = token
	p.mountPath = mountPath
	p.basePath = basePath

	config := vault.DefaultConfig()
	config.Address = addr

	client, err := vault.NewClient(config)
	if err != nil {
		return err
	}

	client.SetToken(token)
	p.client = client

	return nil
}

func (p *VaultProvider) IsInitialized() bool {
	return p.client != nil
}

func (p *VaultProvider) Push(localPath string, remotePath string) error {
	// TODO: Implement push with state management
	return fmt.Errorf("not implemented")
}

func (p *VaultProvider) Pull(remotePath string, localPath string) error {
	// TODO: Implement pull with state management
	return fmt.Errorf("not implemented")
}

func (p *VaultProvider) List(remotePath string) ([]string, error) {
	// TODO: Implement list
	return nil, fmt.Errorf("not implemented")
}

func (p *VaultProvider) Delete(remotePath string) error {
	// TODO: Implement delete
	return fmt.Errorf("not implemented")
}
