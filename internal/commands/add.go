package commands

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/prompt"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Create a new safe in a subdirectory",
	Long: `Add creates a safe: a subdirectory named after it with its own penhan.yaml,
secrets directory, encryption key, and backend credentials. Each safe uses its
name as the backend base path, so several safes can share one backend.

Run it interactively, or pass every option as a flag for scripts and CI.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().String("encryption", "", "Encryption method (gpg/aes/github-gpg)")
	addCmd.Flags().String("github-username", "", "GitHub username (for github-gpg encryption)")
	addCmd.Flags().String("backend", "", "Backend type (vault/file)")
	addCmd.Flags().String("vault-addr", "", "Vault address")
	addCmd.Flags().String("vault-token", "", "Vault token (prefer --vault-token-file)")
	addCmd.Flags().String("vault-token-file", "", "Path to file containing Vault token")
	addCmd.Flags().String("remote-dir", "", "Remote directory (for file backend)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	partial := &prompt.InitAnswers{}

	if len(args) > 0 {
		if err := prompt.ValidateSafeName(args[0]); err != nil {
			return err
		}
		partial.SafeName = args[0]
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
		if v != "vault" && v != "file" {
			return fmt.Errorf("unsupported backend: %s (must be vault or file)", v)
		}
		partial.Backend = v
	}
	if v, _ := cmd.Flags().GetString("vault-addr"); v != "" {
		partial.VaultAddr = v
	}
	if v, _ := cmd.Flags().GetString("remote-dir"); v != "" {
		partial.RemoteDir = v
	}

	// Token resolution: file > flag > prompt.
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

	if !stdinIsTTY() {
		if missing := missingFlags(partial); len(missing) > 0 {
			return fmt.Errorf("non-interactive mode requires all flags; missing: %s", strings.Join(missing, ", "))
		}
	}

	answers, err := prompt.RunInitPrompts(partial)
	if err != nil {
		return err
	}

	if answers.SafeName == "" {
		nameInput := huh.NewInput().
			Title("Safe name").
			Placeholder("myapp").
			Validate(prompt.ValidateSafeName)
		if err := nameInput.Value(&answers.SafeName).Run(); err != nil {
			return err
		}
	}

	if answers.Encryption == "github-gpg" && answers.GitHubUsername == "" {
		return fmt.Errorf("github username is required for github-gpg encryption")
	}
	if answers.Backend == "vault" {
		if err := validateVaultAddress(answers.VaultAddr); err != nil {
			return err
		}
	}

	return createSafe(answers)
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

// missingFlags lists what non-interactive mode still needs before it can
// create a safe without prompting.
func missingFlags(p *prompt.InitAnswers) []string {
	var missing []string
	if p.SafeName == "" {
		missing = append(missing, "safe name (positional arg)")
	}
	if p.Encryption == "" {
		missing = append(missing, "--encryption")
	}
	if p.Encryption == "github-gpg" && p.GitHubUsername == "" {
		missing = append(missing, "--github-username")
	}
	if p.Backend == "" {
		missing = append(missing, "--backend")
	}
	if p.Backend == "vault" {
		if p.VaultAddr == "" {
			missing = append(missing, "--vault-addr")
		}
		if p.VaultToken == "" {
			missing = append(missing, "--vault-token or --vault-token-file")
		}
	}
	return missing
}

// validateVaultAddress rejects addresses the Vault client would only choke on
// much later (e.g. "0.0.0.0:8200" fails at first push with a cryptic URL error).
func validateVaultAddress(addr string) error {
	u, err := url.Parse(addr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid vault address %q: must include a scheme, e.g. http://127.0.0.1:8200", addr)
	}
	return nil
}

// createSafe builds the safe directory: penhan.yaml, the secrets directory,
// the encryption key, backend credentials, and .gitignore entries.
func createSafe(answers *prompt.InitAnswers) error {
	dir := answers.SafeName
	penhanPath := filepath.Join(dir, "penhan.yaml")
	if _, err := os.Stat(penhanPath); err == nil {
		return fmt.Errorf("%s already exists; safe %q is already initialized", penhanPath, dir)
	}

	method := answers.Encryption
	relKeyPath := filepath.Join(".penhan", "keys", method+".key")
	absKeyPath := filepath.Join(dir, relKeyPath)
	encryption := config.EncryptionConfig{Method: method}
	switch method {
	case "gpg":
		encryption.GPG.KeyPath = relKeyPath
	case "github-gpg":
		encryption.GPG.KeyPath = relKeyPath
		encryption.GPG.GitHubUsername = answers.GitHubUsername
	case "aes":
		encryption.AES.KeyPath = relKeyPath
	}
	cfg := &config.Config{
		Encryption: encryption,
		Backend:    buildBackendConfig(answers),
		Secrets: config.SecretsConfig{
			Path:   "secrets/",
			Format: "yaml",
		},
	}

	if err := os.MkdirAll(filepath.Join(dir, ".penhan", "keys"), 0o700); err != nil {
		return fmt.Errorf("create keys directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "secrets"), 0o700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}
	if err := config.Save(cfg, penhanPath); err != nil {
		return err
	}

	if answers.Backend == "vault" {
		tokenPath := filepath.Join(dir, ".penhan", "vault-token")
		if err := os.WriteFile(tokenPath, []byte(answers.VaultToken), 0o600); err != nil {
			return err
		}
	}
	if answers.Backend == "file" {
		if err := os.MkdirAll(filepath.Join(dir, cfg.Backend.File.Path), 0o700); err != nil {
			return fmt.Errorf("create remote directory: %w", err)
		}
	}

	var provider crypto.Provider
	providerArgs := ""
	switch method {
	case "gpg":
		provider = crypto.NewGPGProvider()
	case "github-gpg":
		provider = crypto.NewGitHubGPGProvider()
		providerArgs = answers.GitHubUsername
	case "aes":
		provider = crypto.NewAESProvider()
	}
	if err := provider.Setup(absKeyPath, providerArgs); err != nil {
		return err
	}

	fmt.Printf("✓ Created safe %s\n", dir)
	fmt.Printf("✓ Generated %s key at %s\n", strings.ToUpper(method), absKeyPath)
	fmt.Printf("✓ Created %s\n", penhanPath)

	if err := appendGitignore(gitignoreEntries(dir, answers.Backend)); err != nil {
		return fmt.Errorf("updating .gitignore: %w", err)
	}
	fmt.Printf("✓ Updated .gitignore\n")

	return nil
}

// gitignoreEntries lists the patterns that keep a safe's plaintext secrets,
// keys, and credentials out of git. The .gitignore lives in the project root,
// and any pattern containing a slash is anchored to that directory, so each
// entry is prefixed with the safe directory or git would never match it.
func gitignoreEntries(dir, backend string) []string {
	entries := []string{
		dir + "/secrets/*.yaml",
		dir + "/secrets/*.yml",
		dir + "/secrets/*.json",
		dir + "/.penhan/keys/",
	}
	if backend == "vault" {
		entries = append(entries, dir+"/.penhan/vault-token")
	}
	return entries
}

// buildBackendConfig returns the BackendConfig for the given backend type.
func buildBackendConfig(answers *prompt.InitAnswers) config.BackendConfig {
	switch answers.Backend {
	case "file":
		remoteDir := answers.RemoteDir
		if remoteDir == "" {
			remoteDir = ".penhan/remote"
		}
		return config.BackendConfig{
			Type: "file",
			File: config.FileConfig{Path: remoteDir},
		}
	default:
		return config.BackendConfig{
			Type: "vault",
			Vault: config.VaultConfig{
				Addr:      answers.VaultAddr,
				TokenPath: ".penhan/vault-token",
				MountPath: "secret",
				BasePath:  answers.SafeName,
			},
		}
	}
}

// appendGitignore ensures the given entries are present in .gitignore.
// It creates the file if it does not exist, and appends missing entries
// to an existing file without duplicating lines that are already there.
func appendGitignore(entries []string) error {
	const path = ".gitignore"

	var existing map[string]bool
	data, err := os.ReadFile(path)
	if err == nil {
		existing = make(map[string]bool)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				existing[line] = true
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading .gitignore: %w", err)
	}

	var toAdd []string
	for _, entry := range entries {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening .gitignore: %w", err)
	}
	defer func() { _ = f.Close() }()

	if len(existing) > 0 && !strings.HasSuffix(string(data), "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	for _, entry := range toAdd {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}
	return nil
}
