package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "penhan.yaml")

	content := `
encryption:
  method: aes
  aes:
    key_path: .penhan/keys/aes.key
backend:
  type: vault
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: ""
secrets:
  path: secrets/
  format: yaml
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Encryption.Method != "aes" {
		t.Errorf("Encryption.Method = %q, want %q", cfg.Encryption.Method, "aes")
	}

	if cfg.Backend.Vault.Addr != "https://vault.example.com" {
		t.Errorf("Backend.Vault.Addr = %q, want %q", cfg.Backend.Vault.Addr, "https://vault.example.com")
	}

	if cfg.Secrets.Path != "secrets/" {
		t.Errorf("Secrets.Path = %q, want %q", cfg.Secrets.Path, "secrets/")
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "penhan.yaml")

	original := &Config{
		Encryption: EncryptionConfig{
			Method: "gpg",
			GPG: GPGConfig{
				KeyID: "ABC123",
			},
		},
		Backend: BackendConfig{
			Type: "vault",
			Vault: VaultConfig{
				Addr:      "https://vault.example.com",
				TokenPath: ".penhan/vault-token",
				MountPath: "secret",
			},
		},
		Secrets: SecretsConfig{
			Path:   "secrets/",
			Format: "yaml",
		},
	}

	if err := Save(original, configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.Encryption.Method != original.Encryption.Method {
		t.Errorf("Method = %q, want %q", loaded.Encryption.Method, original.Encryption.Method)
	}
}