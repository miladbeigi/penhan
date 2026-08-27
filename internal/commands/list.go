package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/secrets"
	"github.com/milad/penhan/internal/state"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secrets and their status",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("penhan.yaml")
	if err != nil {
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

	secretsDir := cfg.Secrets.Path
	err = filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			return nil
		}

		vaultPath := secrets.LocalToVault(path, secretsDir)
		entry, exists := s.Secrets[vaultPath]

		status := "new"
		if exists {
			status = entry.Status
		}

		fmt.Printf("  %-40s [%s]\n", path, status)
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}
