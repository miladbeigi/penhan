package commands

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/term"
	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/prompt"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/clientcmd"
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

// defaultRemoteDir is where the file backend writes when no path is configured.
const defaultRemoteDir = ".penhan/remote"

// generateKey creates the safe's key; tests replace it to simulate failures.
var generateKey = crypto.Generate

func init() {
	addCmd.Flags().String("encryption", "", "Encryption method (gpg/aes)")
	addCmd.Flags().String("backend", "", "Backend type (vault/file/kubernetes)")
	addCmd.Flags().String("vault-addr", "", "Vault address")
	addCmd.Flags().String("vault-token", "", "Vault token (prefer --vault-token-file)")
	addCmd.Flags().String("vault-token-file", "", "Path to file containing Vault token")
	addCmd.Flags().String("remote-dir", "", "Remote directory (for file backend)")
	addCmd.Flags().String("kubeconfig", "", "Kubeconfig path (for kubernetes backend; default $KUBECONFIG or ~/.kube/config)")
	addCmd.Flags().String("kube-context", "", "Kubeconfig context (for kubernetes backend; default the current context)")
	addCmd.Flags().String("kube-namespace", "", "Namespace to write Secrets to (for kubernetes backend)")
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
		if !crypto.IsMethod(v) {
			return fmt.Errorf("invalid encryption method: %s (must be gpg or aes)", v)
		}
		partial.Encryption = v
	}
	if v, _ := cmd.Flags().GetString("backend"); v != "" {
		if v != backends.TypeVault && v != backends.TypeFile && v != backends.TypeKubernetes {
			return fmt.Errorf("unsupported backend: %s (must be vault, file, or kubernetes)", v)
		}
		partial.Backend = v
	}
	if v, _ := cmd.Flags().GetString("vault-addr"); v != "" {
		partial.VaultAddr = v
	}
	if v, _ := cmd.Flags().GetString("remote-dir"); v != "" {
		partial.RemoteDir = v
	}
	if v, _ := cmd.Flags().GetString("kubeconfig"); v != "" {
		// Commands run inside the safe directory, so a path relative to
		// where add ran would no longer resolve.
		abs, err := filepath.Abs(v)
		if err != nil {
			return err
		}
		partial.Kubeconfig = abs
	}
	if v, _ := cmd.Flags().GetString("kube-context"); v != "" {
		partial.KubeContext = v
	}
	if v, _ := cmd.Flags().GetString("kube-namespace"); v != "" {
		partial.KubeNamespace = v
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

	switch answers.Backend {
	case backends.TypeVault:
		if err := validateVaultAddress(answers.VaultAddr); err != nil {
			return err
		}
	case backends.TypeKubernetes:
		if errs := validation.IsDNS1123Label(answers.KubeNamespace); len(errs) > 0 {
			return fmt.Errorf("invalid kubernetes namespace %q: %s", answers.KubeNamespace, strings.Join(errs, "; "))
		}
		kubeContext, err := resolveKubeContext(answers.Kubeconfig, answers.KubeContext)
		if err != nil {
			return err
		}
		answers.KubeContext = kubeContext
	}

	return createSafe(answers)
}

func stdinIsTTY() bool {
	return term.IsTerminal(os.Stdin.Fd())
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
	if p.Backend == "" {
		missing = append(missing, "--backend")
	}
	if p.Backend == backends.TypeKubernetes && p.KubeNamespace == "" {
		missing = append(missing, "--kube-namespace")
	}
	if p.Backend == backends.TypeVault {
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

// resolveKubeContext returns the context the safe will be pinned to: the
// requested one if it exists in the kubeconfig, otherwise the current one
// (or a prompt to pick one when interactive and there is a choice). Pinning
// it in penhan.yaml means switching kubectl contexts later can never send a
// push to a different cluster.
func resolveKubeContext(kubeconfig, requested string) (string, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.ExplicitPath = kubeconfig
	kcfg, err := rules.Load()
	if err != nil {
		return "", fmt.Errorf("load kubeconfig: %w", err)
	}

	if requested != "" {
		if _, ok := kcfg.Contexts[requested]; !ok {
			return "", fmt.Errorf("kubeconfig has no context %q", requested)
		}
		return requested, nil
	}

	names := make([]string, 0, len(kcfg.Contexts))
	for name := range kcfg.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 1 && stdinIsTTY() {
		return prompt.SelectKubeContext(names, kcfg.CurrentContext)
	}
	if kcfg.CurrentContext == "" {
		return "", fmt.Errorf("kubeconfig has no current context; pass --kube-context")
	}
	return kcfg.CurrentContext, nil
}

// createSafe builds the safe directory: penhan.yaml, the secrets directory,
// the encryption key, backend credentials, and .gitignore entries.
func createSafe(answers *prompt.InitAnswers) (retErr error) {
	dir := answers.SafeName
	penhanPath := filepath.Join(dir, "penhan.yaml")
	if _, err := os.Stat(penhanPath); err == nil {
		return fmt.Errorf("%s already exists; safe %q is already initialized", penhanPath, dir)
	}
	created := false
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		created = true
	} else if err != nil {
		return fmt.Errorf("check safe directory: %w", err)
	}
	defer func() {
		if retErr != nil && created {
			_ = os.RemoveAll(dir)
		}
	}()

	method := answers.Encryption
	relKeyPath := filepath.Join(".penhan", "keys", method+".key")
	absKeyPath := filepath.Join(dir, relKeyPath)
	encryption := config.EncryptionConfig{Method: method}
	switch method {
	case crypto.MethodGPG:
		encryption.GPG.KeyPath = relKeyPath
	case crypto.MethodAES:
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

	if err := generateKey(method, absKeyPath); err != nil {
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
	case backends.TypeKubernetes:
		return config.BackendConfig{
			Type: backends.TypeKubernetes,
			Kubernetes: config.KubernetesConfig{
				Kubeconfig: answers.Kubeconfig,
				Context:    answers.KubeContext,
				Namespace:  answers.KubeNamespace,
				Safe:       answers.SafeName,
			},
		}
	case backends.TypeFile:
		remoteDir := answers.RemoteDir
		if remoteDir == "" {
			remoteDir = defaultRemoteDir
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
