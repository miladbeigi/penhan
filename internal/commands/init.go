package commands

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/charmbracelet/huh"
	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/crypto"
	"github.com/milad/penhan/internal/state"
	"github.com/spf13/cobra"
)

// appendGitignore ensures the given entries are present in .gitignore.
// It creates the file if it does not exist, and appends missing entries
// to an existing file without duplicating lines that are already there.
func appendGitignore(entries []string) error {
	const path = ".gitignore"

	var existing map[string]bool
	data, err := os.ReadFile(path)
	if err == nil {
		existing = make(map[string]bool)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				existing[line] = true
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	var toAdd []string
	for _, entry := range entries {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer func() { _ = f.Close() }()

	// Add leading newline if file already has content and doesn't end with one
	if len(existing) > 0 {
		content := string(data)
		if !strings.HasSuffix(content, "\n") {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	}

	for _, entry := range toAdd {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// validateVaultAddress rejects addresses the Vault client would only choke on
// much later (e.g. "0.0.0.0:8200" fails at first push with a cryptic URL error).
func validateVaultAddress(addr string) error {
	u, err := url.Parse(addr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid vault address %q: must include a scheme, e.g. http://127.0.0.1:8200", addr)
	}
	return nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize penhan in the current directory",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat("penhan.yaml"); err == nil {
		return fmt.Errorf("penhan.yaml already exists; this directory is already initialized")
	}

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

	if err := validateVaultAddress(vaultAddr); err != nil {
		return err
	}

	fmt.Print("Vault token: ")
	vaultToken, _ := reader.ReadString('\n')
	vaultToken = strings.TrimSpace(vaultToken)

	// Create config
	keyPath := filepath.Join(".penhan", "keys", method+".key")
	encryption := config.EncryptionConfig{Method: method}
	switch method {
	case "gpg":
		encryption.GPG.KeyPath = keyPath
	case "aes":
		encryption.AES.KeyPath = keyPath
	}
	cfg := &config.Config{
		Encryption: encryption,
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
	if err := os.MkdirAll(".penhan/keys", 0o700); err != nil {
		return fmt.Errorf("create keys directory: %w", err)
	}
	if err := os.MkdirAll("secrets", 0o700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}

	// Save vault token
	if err := os.WriteFile(".penhan/vault-token", []byte(vaultToken), 0o600); err != nil {
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

	if err := provider.Setup(keyPath, ""); err != nil {
		return err
	}

	// Create state file
	statePath := filepath.Join(".penhan", "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		s := state.NewState()
		if err := state.Save(s, statePath); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Initialized penhan in current directory\n")
	fmt.Printf("✓ Generated %s key at %s\n", strings.ToUpper(method), keyPath)
	fmt.Printf("✓ Created penhan.yaml\n")

	secretsDir := strings.TrimSuffix(cfg.Secrets.Path, "/")
	gitignoreEntries := []string{
		secretsDir + "/*.yaml",
		secretsDir + "/*.yml",
		".penhan/keys/",
		".penhan/vault-token",
		".penhan/config.yaml",
		".penhan/state.json",
	}
	if err := appendGitignore(gitignoreEntries); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}
	fmt.Printf("✓ Updated .gitignore\n")

	return nil
}
