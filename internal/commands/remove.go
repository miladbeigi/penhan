package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milad/penhan/internal/backends"
	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/secrets"
	"github.com/milad/penhan/internal/state"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a secret from local and Vault",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	filePath := filepath.Join(cfg.Secrets.Path, name+"."+cfg.Secrets.Format)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("secret not found: %w", err)
	}

	fmt.Printf("Remove secret %s? (y/N) ", name)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer != "y" && answer != "Y" {
		fmt.Println("Aborted")
		return nil
	}

	if err := os.Remove(filePath); err != nil {
		return err
	}

	encPath := filePath + ".enc"
	if _, err := os.Stat(encPath); err == nil {
		if err := os.Remove(encPath); err != nil {
			return fmt.Errorf("removing encrypted file: %w", err)
		}
	}

	backend := backends.NewVaultProvider()
	token, err := os.ReadFile(cfg.Backend.Vault.TokenPath)
	if err != nil {
		return err
	}
	if err := backend.Setup(cfg.Backend.Vault.Addr, string(token), cfg.Backend.Vault.MountPath, cfg.Backend.Vault.BasePath); err != nil {
		return err
	}

	vaultPath := secrets.LocalToVault(filePath, cfg.Secrets.Path)
	if err := backend.Delete(vaultPath); err != nil {
		return err
	}

	statePath := filepath.Join(".penhan", "state.json")
	s, err := state.Load(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			s = state.NewState()
		} else {
			return fmt.Errorf("load state: %w", err)
		}
	}

	delete(s.Secrets, vaultPath)
	if err := state.Save(s, statePath); err != nil {
		return err
	}

	fmt.Printf("✓ Removed secret: %s\n", name)
	return nil
}
