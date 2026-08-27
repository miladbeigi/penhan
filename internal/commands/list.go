package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/secrets"
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
	s, err := loadStateOrNew(statePath)
	if err != nil {
		return err
	}

	// Collect one display path per secret; when both the plaintext and the
	// encrypted file exist, show the plaintext (the editable copy).
	secretsDir := cfg.Secrets.Path
	display := make(map[string]string)
	err = filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
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
		return err
	}

	paths := make([]string, 0, len(display))
	for vaultPath := range display {
		paths = append(paths, vaultPath)
	}
	sort.Strings(paths)

	for _, vaultPath := range paths {
		status := "new"
		if entry, exists := s.Secrets[vaultPath]; exists {
			status = entry.Status
		}
		fmt.Printf("  %-40s [%s]\n", display[vaultPath], status)
	}

	return nil
}
