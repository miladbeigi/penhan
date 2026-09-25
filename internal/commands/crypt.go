package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/secrets"
	"github.com/spf13/cobra"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file|dir]...",
	Short: "Encrypt secret files in place",
	Long: `Encrypt replaces each plaintext secret file (.yaml, .yml, .json) with an
encrypted .enc copy for committing to git. With no arguments it encrypts the
whole secrets directory. Other files are left alone.

Encryption is randomized, so encrypting the same plaintext twice gives
different bytes. When the existing .enc file already decrypts to exactly the
plaintext, it is kept byte for byte, so git only shows secrets that changed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCrypt(args, encryptFile)
	},
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt [file|dir]...",
	Short: "Decrypt secret files in place",
	Long: `Decrypt writes the plaintext of each .enc file next to it, for editing.
The .enc file is kept, so encrypt can tell whether anything changed. With no
arguments it decrypts the whole secrets directory. It refuses to overwrite a
plaintext file whose content differs from the encrypted copy, so local edits
are never lost.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCrypt(args, decryptFile)
	},
}

func init() {
	rootCmd.AddCommand(encryptCmd, decryptCmd)
}

// runCrypt applies fn to every file under args (default: the secrets directory).
func runCrypt(args []string, fn func(path string, provider crypto.Provider) error) error {
	cfg, provider, err := openSafe()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{cfg.Secrets.Path}
	}

	for _, arg := range args {
		err := filepath.WalkDir(arg, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			return fn(p, provider)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func encryptFile(path string, provider crypto.Provider) error {
	if strings.HasSuffix(path, ".enc") {
		return nil
	}
	if !secrets.IsSecretFile(path) {
		fmt.Printf("  Skipping (not a secret file): %s\n", path)
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	status := "Unchanged"
	if !encryptedCopyMatches(path+".enc", data, provider) {
		encrypted, err := provider.Encrypt(data)
		if err != nil {
			return fmt.Errorf("encrypt %s: %w", path, err)
		}
		// The .enc copy is meant to be committed, so it is not owner-only.
		if err := os.WriteFile(path+".enc", encrypted, 0o644); err != nil {
			return err
		}
		status = "Encrypted"
	}

	// Encrypting at rest means the plaintext must not linger next to it.
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove plaintext file: %w", err)
	}

	fmt.Printf("  %s: %s\n", status, path)
	return nil
}

// encryptedCopyMatches reports whether encPath exists and decrypts to exactly
// plaintext. Re-encrypting would then only churn the file in git, since each
// encryption uses a fresh random nonce (AES) or session key (GPG).
func encryptedCopyMatches(encPath string, plaintext []byte, provider crypto.Provider) bool {
	existing, err := os.ReadFile(encPath)
	if err != nil {
		return false
	}
	current, err := provider.Decrypt(existing)
	return err == nil && bytes.Equal(current, plaintext)
}

func decryptFile(path string, provider crypto.Provider) error {
	if !strings.HasSuffix(path, ".enc") {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decrypted, err := provider.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt %s: %w", path, err)
	}

	// The .enc file stays: encrypt compares against it to avoid rewriting
	// unchanged secrets, and the plaintext is gitignored.
	decPath := strings.TrimSuffix(path, ".enc")
	existing, err := os.ReadFile(decPath)
	switch {
	case err == nil && bytes.Equal(existing, decrypted):
		return nil // already decrypted
	case err == nil:
		return fmt.Errorf("%s already exists with different content; encrypt or remove it before decrypting %s", decPath, path)
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	if err := os.WriteFile(decPath, decrypted, 0o600); err != nil {
		return err
	}

	fmt.Printf("  Decrypted: %s\n", decPath)
	return nil
}
