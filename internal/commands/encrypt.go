package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/spf13/cobra"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file|dir]",
	Short: "Encrypt secret files in place",
	RunE:  runEncrypt,
}

func init() {
	rootCmd.AddCommand(encryptCmd)
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	cfg, err := loadSafeConfig()
	if err != nil {
		return err
	}

	provider, err := newCryptoProvider(cfg)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		args = []string{cfg.Secrets.Path}
	}

	for _, arg := range args {
		if err := encryptPath(arg, provider); err != nil {
			return err
		}
	}

	return nil
}

func encryptPath(path string, provider crypto.Provider) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return encryptFile(p, provider)
			}
			return nil
		})
	}

	return encryptFile(path, provider)
}

func encryptFile(path string, provider crypto.Provider) error {
	if strings.HasSuffix(path, ".enc") {
		fmt.Printf("  Skipping (already encrypted): %s\n", path)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	encrypted, err := provider.Encrypt(data)
	if err != nil {
		return err
	}

	encPath := path + ".enc"
	if err := os.WriteFile(encPath, encrypted, 0o644); err != nil {
		return err
	}

	// Mirror decrypt (which removes the .enc): encrypting at rest means the
	// plaintext must not linger next to the encrypted file.
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove plaintext file: %w", err)
	}

	fmt.Printf("  Encrypted: %s\n", path)
	return nil
}
