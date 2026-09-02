package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/crypto"
	"github.com/spf13/cobra"
)

var decryptCmd = &cobra.Command{
	Use:   "decrypt [file|dir]",
	Short: "Decrypt secret files in place",
	RunE:  runDecrypt,
}

func init() {
	rootCmd.AddCommand(decryptCmd)
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	var provider crypto.Provider
	switch cfg.Encryption.Method {
	case "gpg":
		provider = crypto.NewGPGProvider()
		if err := provider.Setup(cfg.Encryption.GPG.KeyPath, ""); err != nil {
			return err
		}
	case "github-gpg":
		return fmt.Errorf("decrypt is not supported with github-gpg method (seal-only provider)")
	case "aes":
		provider = crypto.NewAESProvider()
		if err := provider.Setup(cfg.Encryption.AES.KeyPath, ""); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported encryption method: %s", cfg.Encryption.Method)
	}

	if len(args) == 0 {
		args = []string{cfg.Secrets.Path}
	}

	for _, arg := range args {
		if err := decryptPath(arg, provider); err != nil {
			return err
		}
	}

	return nil
}

func decryptPath(path string, provider crypto.Provider) error {
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
				return decryptFile(p, provider)
			}
			return nil
		})
	}

	return decryptFile(path, provider)
}

func decryptFile(path string, provider crypto.Provider) error {
	if !strings.HasSuffix(path, ".enc") {
		fmt.Printf("  Skipping (not encrypted): %s\n", path)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	decrypted, err := provider.Decrypt(data)
	if err != nil {
		return err
	}

	decPath := strings.TrimSuffix(path, ".enc")
	if err := os.WriteFile(decPath, decrypted, 0o644); err != nil {
		return err
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove encrypted file: %w", err)
	}

	fmt.Printf("  Decrypted: %s\n", decPath)
	return nil
}
