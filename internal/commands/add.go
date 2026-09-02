package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/secrets"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new secret",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	filePath := filepath.Join(cfg.Secrets.Path, name+"."+cfg.Secrets.Format)

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("secret already exists: %s", filePath)
	}

	data := map[string]string{
		"key": "value",
	}

	if err := secrets.WriteFile(filePath, data); err != nil {
		return err
	}

	fmt.Printf("✓ Created secret: %s\n", filePath)
	return nil
}
