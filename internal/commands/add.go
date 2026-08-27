package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/secrets"
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
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("secret already exists: %s", filePath)
	}

	fmt.Printf("Enter value for %s: ", name)
	reader := bufio.NewReader(os.Stdin)
	value, _ := reader.ReadString('\n')
	value = strings.TrimSpace(value)

	data := map[string]string{
		"value": value,
	}

	if err := secrets.WriteFile(filePath, data); err != nil {
		return err
	}

	fmt.Printf("✓ Created secret: %s\n", filePath)
	return nil
}
