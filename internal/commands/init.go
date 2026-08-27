package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milad/penhan/internal/crypto"
	"github.com/milad/penhan/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize penhan in the current directory",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Ask for encryption method
	fmt.Print("Encryption method (gpg/aes): ")
	method, _ := reader.ReadString('\n')
	method = strings.TrimSpace(method)

	if method != "gpg" && method != "aes" {
		return fmt.Errorf("invalid encryption method: %s", method)
	}

	// Ask for backend
	fmt.Print("Backend type (vault): ")
	backendType, _ := reader.ReadString('\n')
	backendType = strings.TrimSpace(backendType)

	if backendType != "vault" {
		return fmt.Errorf("unsupported backend: %s", backendType)
	}

	// Ask for Vault config
	fmt.Print("Vault address: ")
	vaultAddr, _ := reader.ReadString('\n')
	vaultAddr = strings.TrimSpace(vaultAddr)

	fmt.Print("Vault token: ")
	vaultToken, _ := reader.ReadString('\n')
	vaultToken = strings.TrimSpace(vaultToken)

	// Create config
	cfg := &config.Config{
		Encryption: config.EncryptionConfig{
			Method: method,
		},
		Backend: config.BackendConfig{
			Type: "vault",
			Vault: config.VaultConfig{
				Addr:      vaultAddr,
				TokenPath: ".penhan/vault-token",
				MountPath: "secret",
			},
		},
		Secrets: config.SecretsConfig{
			Path:   "secrets/",
			Format: "yaml",
		},
	}

	// Save config
	if err := config.Save(cfg, "penhan.yaml"); err != nil {
		return err
	}

	// Create directories
	if err := os.MkdirAll(".penhan/keys", 0700); err != nil {
		return fmt.Errorf("create keys directory: %w", err)
	}
	if err := os.MkdirAll("secrets", 0700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}

	// Save vault token
	if err := os.WriteFile(".penhan/vault-token", []byte(vaultToken), 0600); err != nil {
		return err
	}

	// Setup encryption provider
	var provider crypto.Provider
	switch method {
	case "gpg":
		provider = crypto.NewGPGProvider()
	case "aes":
		provider = crypto.NewAESProvider()
	}

	keyPath := filepath.Join(".penhan", "keys", method+".key")
	if err := provider.Setup(keyPath, ""); err != nil {
		return err
	}

	// Create state file
	statePath := filepath.Join(".penhan", "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		if err := os.WriteFile(statePath, []byte(`{"version":1,"secrets":{}}`), 0644); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Initialized penhan in current directory\n")
	fmt.Printf("✓ Generated %s key at %s\n", strings.ToUpper(method), keyPath)
	fmt.Printf("✓ Created penhan.yaml\n")

	return nil
}
