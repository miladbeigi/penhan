package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Encryption EncryptionConfig `yaml:"encryption"`
	Backend    BackendConfig    `yaml:"backend"`
	Secrets    SecretsConfig    `yaml:"secrets"`
}

type EncryptionConfig struct {
	Method string    `yaml:"method"`
	GPG    GPGConfig `yaml:"gpg,omitempty"`
	AES    AESConfig `yaml:"aes,omitempty"`
}

type GPGConfig struct {
	KeyPath string `yaml:"key_path,omitempty"`
}

type AESConfig struct {
	KeyPath string `yaml:"key_path,omitempty"`
}

// KeyPath returns the key file path configured for the selected method.
func (e EncryptionConfig) KeyPath() string {
	if e.Method == "gpg" {
		return e.GPG.KeyPath
	}
	return e.AES.KeyPath
}

type BackendConfig struct {
	Type       string           `yaml:"type"`
	Vault      VaultConfig      `yaml:"vault,omitempty"`
	File       FileConfig       `yaml:"file,omitempty"`
	Kubernetes KubernetesConfig `yaml:"kubernetes,omitempty"`
}

type VaultConfig struct {
	Addr      string `yaml:"addr"`
	TokenPath string `yaml:"token_path"`
	MountPath string `yaml:"mount_path"`
	BasePath  string `yaml:"base_path"`
}

type FileConfig struct {
	Path string `yaml:"path"`
}

type KubernetesConfig struct {
	Kubeconfig string `yaml:"kubeconfig,omitempty"` // empty: $KUBECONFIG or ~/.kube/config
	Context    string `yaml:"context"`
	Namespace  string `yaml:"namespace"`
	Safe       string `yaml:"safe"` // recorded on each Secret to mark ownership
}

type SecretsConfig struct {
	Path   string `yaml:"path"`
	Format string `yaml:"format"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
