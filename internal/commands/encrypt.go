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

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file|dir]",
	Short: "Encrypt secret files in place",
	RunE:  runEncrypt,
}

func init() {
	rootCmd.AddCommand(encryptCmd)
}

func runEncrypt(cmd *cobra.Command, args []string) error {
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
	case "aes":
		provider = crypto.NewAESProvider()
		if err := provider.Setup(cfg.Encryption.AES.KeyPath, ""); err != nil {
			return err
		}
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
	if err := os.WriteFile(encPath, encrypted, 0644); err != nil {
		return err
	}

	fmt.Printf("  Encrypted: %s\n", path)
	return nil
}
