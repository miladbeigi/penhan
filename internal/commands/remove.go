package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/secrets"
	"github.com/milad/penhan/internal/state"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove [path]",
	Short: "Remove a secret from local and Vault",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRemove,
}

func init() {
	removeCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}

// resolveSecretPair takes a user-supplied path and returns the canonical
// plaintext path and the encrypted variant. It validates that the resolved
// path is inside the secrets directory.
func resolveSecretPair(input, secretsDir string) (plaintext, enc string, err error) {
	input = filepath.Clean(input)
	if strings.HasSuffix(input, ".enc") {
		plaintext = strings.TrimSuffix(input, ".enc")
	} else {
		plaintext = input
	}
	enc = plaintext + ".enc"

	if !strings.HasPrefix(plaintext, secretsDir) {
		return "", "", fmt.Errorf("path is outside the secrets directory: %s", input)
	}

	return plaintext, enc, nil
}

// removeSecretFilesAndState deletes both file variants, the Vault entry, and
// the state entry for a given plaintext path.
func removeSecretFilesAndState(plaintext, encPath, secretsDir string, force bool, cfg *config.Config) error {
	plaintextExists := fileExists(plaintext)
	encExists := fileExists(encPath)

	if !plaintextExists && !encExists {
		return fmt.Errorf("secret not found: %s (neither plaintext nor encrypted variant exists)", plaintext)
	}

	// Determine what will be removed
	var targets []string
	if plaintextExists {
		targets = append(targets, plaintext)
	}
	if encExists {
		targets = append(targets, encPath)
	}

	vaultPath := secrets.LocalToVault(plaintext, secretsDir)
	targets = append(targets, fmt.Sprintf("vault entry: %s", vaultPath))

	if !force {
		fmt.Println("The following will be removed:")
		for _, t := range targets {
			fmt.Printf("  - %s\n", t)
		}
		fmt.Print("\nConfirm? (y/N) ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)

		if answer != "y" && answer != "Y" {
			fmt.Println("Aborted")
			return nil
		}
	}

	// Remove files
	if plaintextExists {
		if err := os.Remove(plaintext); err != nil {
			return fmt.Errorf("removing plaintext: %w", err)
		}
	}
	if encExists {
		if err := os.Remove(encPath); err != nil {
			return fmt.Errorf("removing encrypted: %w", err)
		}
	}

	// Remove from Vault
	backend, err := newVaultBackend(cfg)
	if err != nil {
		return err
	}
	if err := backend.Delete(vaultPath); err != nil {
		return err
	}

	// Remove from state
	statePath := filepath.Join(".penhan", "state.json")
	s, err := loadStateOrNew(statePath)
	if err != nil {
		return err
	}
	delete(s.Secrets, vaultPath)
	if err := state.Save(s, statePath); err != nil {
		return err
	}

	displayName := filepath.Base(plaintext)
	fmt.Printf("✓ Removed secret: %s\n", displayName)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// discoverSecrets walks the secrets directory and returns one entry per
// secret pair, sorted by display path. The same logic used by `penhan list`.
func discoverSecrets(secretsDir string) ([]string, error) {
	display := make(map[string]string)
	err := filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		name := path
		isEnc := strings.HasSuffix(name, ".enc")
		if isEnc {
			name = strings.TrimSuffix(name, ".enc")
		}
		ext := filepath.Ext(name)
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil
		}

		vaultPath := secrets.LocalToVault(name, secretsDir)
		if _, seen := display[vaultPath]; seen && isEnc {
			return nil
		}
		display[vaultPath] = path
		return nil
	})
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(display))
	for vaultPath := range display {
		paths = append(paths, vaultPath)
	}
	sort.Strings(paths)
	return paths, nil
}

func runRemove(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	secretsDir := cfg.Secrets.Path

	// Interactive mode: no arguments
	if len(args) == 0 {
		return runRemoveInteractive(cfg, secretsDir, force)
	}

	// Single path argument
	input := args[0]
	plaintext, encPath, err := resolveSecretPair(input, secretsDir)
	if err != nil {
		return err
	}

	return removeSecretFilesAndState(plaintext, encPath, secretsDir, force, cfg)
}

func runRemoveInteractive(cfg *config.Config, secretsDir string, force bool) error {
	// Check for TTY
	if info, err := os.Stdin.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
		return fmt.Errorf("interactive mode requires a TTY; pass a file path or use --force with a path")
	}

	entries, err := discoverSecrets(secretsDir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No secrets found.")
		return nil
	}

	// Display numbered list
	for i, vaultPath := range entries {
		fmt.Printf("  %d) %s\n", i+1, vaultPath)
	}

	fmt.Print("\nEnter number to remove: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	idx, err := strconv.Atoi(answer)
	if err != nil || idx < 1 || idx > len(entries) {
		return fmt.Errorf("invalid selection: %s", answer)
	}

	vaultPath := entries[idx-1]
	plaintext := filepath.Join(secretsDir, vaultPath+"."+cfg.Secrets.Format)
	encPath := plaintext + ".enc"

	return removeSecretFilesAndState(plaintext, encPath, secretsDir, force, cfg)
}
