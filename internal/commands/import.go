package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/miladbeigi/penhan/internal/secrets"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var importCmd = &cobra.Command{
	Use:   "import [secret]...",
	Short: "Bring secrets that already exist in the backend into the safe",
	Long: `Import takes over secrets that were created outside penhan (kubernetes
backend only). With no arguments it lists every Secret in the namespace and
whether it can be imported. Otherwise it writes each named Secret, or every
importable one with --all, to secrets/<name>.yaml.enc, encrypted with the
safe's key so the plaintext never touches disk, and marks the Secret as
managed by this safe. Its data is not changed.

Secrets managed by Helm, Argo CD, a controller, or another safe are never
imported, and neither are non-Opaque types such as image pull secrets.`,
	RunE: runImport,
}

func init() {
	importCmd.Flags().Bool("all", false, "Import every importable secret")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if all && len(args) > 0 {
		return fmt.Errorf("pass secret names or --all, not both")
	}

	cfg, provider, err := openSafe()
	if err != nil {
		return err
	}
	backend, err := newBackend(cfg, provider)
	if err != nil {
		return err
	}
	importer, ok := backend.(backends.Importer)
	if !ok {
		return fmt.Errorf("import is not supported for the %s backend", cfg.Backend.Type)
	}

	candidates, err := importer.ImportCandidates()
	if err != nil {
		return err
	}

	if !all && len(args) == 0 {
		printImportCandidates(cfg, candidates)
		return nil
	}

	names := args
	if all {
		names = nil
		for _, c := range candidates {
			if c.Reason == "" && localSecretFile(cfg, c.Name) == "" {
				names = append(names, c.Name)
			}
		}
		if len(names) == 0 {
			fmt.Println("Nothing to import.")
			return nil
		}
	}

	for _, name := range names {
		if err := importSecret(cfg, provider, importer, name); err != nil {
			return err
		}
	}
	fmt.Printf("\nImported %d secret(s). Commit the .enc files; `penhan check` should report them unchanged.\n", len(names))
	return nil
}

func printImportCandidates(cfg *config.Config, candidates []backends.ImportCandidate) {
	if len(candidates) == 0 {
		fmt.Println("No secrets found.")
		return
	}
	ready := 0
	for _, c := range candidates {
		switch {
		case c.Reason != "":
			fmt.Printf("  %-9s %s: %s\n", "skip", c.Name, c.Reason)
		case localSecretFile(cfg, c.Name) != "":
			fmt.Printf("  %-9s %s: already in %s\n", "present", c.Name, localSecretFile(cfg, c.Name))
		default:
			fmt.Printf("  %-9s %s (%d key(s))\n", "ready", c.Name, c.Keys)
			ready++
		}
	}
	fmt.Printf("\n%d secret(s) ready to import", ready)
	if ready > 0 {
		fmt.Print(": run `penhan import <name>...` or `penhan import --all`")
	}
	fmt.Println()
}

func importSecret(cfg *config.Config, provider crypto.Provider, importer backends.Importer, name string) error {
	if existing := localSecretFile(cfg, name); existing != "" {
		return fmt.Errorf("cannot import %s: %s already exists", name, existing)
	}
	path, content, err := importer.ReadForImport(name)
	if err != nil {
		return err
	}

	plaintext, err := secretYAML(content)
	if err != nil {
		return fmt.Errorf("import %s: %w", name, err)
	}
	encrypted, err := provider.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt %s: %w", name, err)
	}

	target := filepath.Join(cfg.Secrets.Path, path+".yaml.enc")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(target, encrypted, 0o644); err != nil {
		return err
	}
	if err := importer.Adopt(path, content); err != nil {
		_ = os.Remove(target)
		return err
	}

	fmt.Printf("  Imported: %s → %s\n", name, target)
	return nil
}

// secretYAML renders backend content as a secret file, and proves the file
// parses back to exactly the same key-value pairs before anything is written.
func secretYAML(content []byte) ([]byte, error) {
	var kv map[string]string
	if err := json.Unmarshal(content, &kv); err != nil {
		return nil, err
	}
	out, err := yaml.Marshal(kv)
	if err != nil {
		return nil, err
	}

	parsed, err := secrets.Parse(out, ".yaml")
	if err != nil {
		return nil, fmt.Errorf("rendered file does not parse: %w", err)
	}
	roundtrip, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(roundtrip, canonicalJSON(content)) {
		return nil, fmt.Errorf("a value cannot be represented exactly in a YAML secret file")
	}
	return out, nil
}

// localSecretFile returns the local file (plaintext or encrypted) holding the
// secret at path, or "" when the safe has none.
func localSecretFile(cfg *config.Config, path string) string {
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		for _, suffix := range []string{"", ".enc"} {
			p := filepath.Join(cfg.Secrets.Path, path+ext+suffix)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}
