package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/prompt"
	"github.com/spf13/cobra"
)

var safeCmd = &cobra.Command{
	Use:   "safe",
	Short: "Manage named secret safes",
}

var safeAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Create and initialize a new safe",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSafeAdd,
}

var safeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List safes in the current directory",
	RunE:  runSafeList,
}

func init() {
	safeAddCmd.Flags().String("encryption", "", "Encryption method (gpg/aes/github-gpg)")
	safeAddCmd.Flags().String("github-username", "", "GitHub username (for github-gpg encryption)")
	safeAddCmd.Flags().String("backend", "", "Backend type (vault)")
	safeAddCmd.Flags().String("vault-addr", "", "Vault address")
	safeAddCmd.Flags().String("vault-token", "", "Vault token (prefer --vault-token-file)")
	safeAddCmd.Flags().String("vault-token-file", "", "Path to file containing Vault token")

	safeCmd.AddCommand(safeAddCmd)
	safeCmd.AddCommand(safeListCmd)
	rootCmd.AddCommand(safeCmd)
}

func runSafeAdd(cmd *cobra.Command, args []string) error {
	partial := &prompt.InitAnswers{}

	if len(args) > 0 {
		name := args[0]
		if err := prompt.ValidateSafeName(name); err != nil {
			return err
		}
		partial.SafeName = name
	}

	if v, _ := cmd.Flags().GetString("encryption"); v != "" {
		if v != "gpg" && v != "aes" && v != "github-gpg" {
			return fmt.Errorf("invalid encryption method: %s (must be gpg, aes, or github-gpg)", v)
		}
		partial.Encryption = v
	}
	if v, _ := cmd.Flags().GetString("github-username"); v != "" {
		partial.GitHubUsername = v
	}
	if v, _ := cmd.Flags().GetString("backend"); v != "" {
		if v != "vault" {
			return fmt.Errorf("unsupported backend: %s (only vault is supported)", v)
		}
		partial.Backend = v
	}
	if v, _ := cmd.Flags().GetString("vault-addr"); v != "" {
		partial.VaultAddr = v
	}

	if v, _ := cmd.Flags().GetString("vault-token-file"); v != "" {
		data, err := os.ReadFile(v)
		if err != nil {
			return fmt.Errorf("reading vault token file: %w", err)
		}
		partial.VaultToken = strings.TrimSpace(string(data))
	} else if v, _ := cmd.Flags().GetString("vault-token"); v != "" {
		fmt.Fprintln(os.Stderr, "\033[33mWarning: --vault-token is visible in shell history. Prefer --vault-token-file.\033[0m")
		partial.VaultToken = v
	}

	if info, err := os.Stdin.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
		var missing []string
		if partial.SafeName == "" {
			missing = append(missing, "safe name (positional arg)")
		}
		if partial.Encryption == "" {
			missing = append(missing, "--encryption")
		}
		if partial.Encryption == "github-gpg" && partial.GitHubUsername == "" {
			missing = append(missing, "--github-username")
		}
		if partial.Backend == "" {
			missing = append(missing, "--backend")
		}
		if partial.VaultAddr == "" {
			missing = append(missing, "--vault-addr")
		}
		if partial.VaultToken == "" {
			missing = append(missing, "--vault-token or --vault-token-file")
		}
		if len(missing) > 0 {
			return fmt.Errorf("non-interactive mode requires all flags; missing: %s", strings.Join(missing, ", "))
		}
	}

	answers, err := prompt.RunInitPrompts(partial)
	if err != nil {
		return err
	}

	// If still no safe name, prompt interactively
	if answers.SafeName == "" {
		nameInput := huh.NewInput().
			Title("Safe name").
			Placeholder("myapp").
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("safe name is required")
				}
				return prompt.ValidateSafeName(s)
			})
		if err := nameInput.Value(&answers.SafeName).Run(); err != nil {
			return err
		}
	}

	if answers.Encryption == "github-gpg" && answers.GitHubUsername == "" {
		return fmt.Errorf("github username is required for github-gpg encryption")
	}

	if err := validateVaultAddress(answers.VaultAddr); err != nil {
		return err
	}

	dir := answers.SafeName
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating safe directory: %w", err)
	}

	return initializeSafe(dir, answers)
}

func runSafeList(cmd *cobra.Command, args []string) error {
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading current directory: %w", err)
	}

	var found bool
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfgPath := filepath.Join(e.Name(), "penhan.yaml")
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			continue
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s (invalid config: %v)\n", e.Name(), err)
			continue
		}

		fmt.Printf("%-20s %s\n", e.Name(), cfg.Backend.Type)
		found = true
	}

	if !found {
		fmt.Println("No safes found in current directory.")
	}
	return nil
}
