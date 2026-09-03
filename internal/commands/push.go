package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push local secrets to the backend",
	Long: `Push runs the same comparison as check, then writes every secret that is
new or changed to the backend. Secrets whose hash already matches the backend
are skipped. Each secret is reported as it is processed.`,
	Args: cobra.NoArgs,
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	cfg, err := loadSafeConfig()
	if err != nil {
		return err
	}

	provider, err := newCryptoProvider(cfg)
	if err != nil {
		return err
	}

	backend, err := newBackend(cfg, provider)
	if err != nil {
		return err
	}

	results, err := checkSecrets(cfg, provider, backend)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No secrets found.")
		return nil
	}

	pushed := 0
	for _, r := range results {
		if !r.NeedsPush() {
			fmt.Printf("  Unchanged: %s\n", r.Secret.Path)
			continue
		}
		if err := backend.Push(r.Secret.Content, r.Secret.Path); err != nil {
			return fmt.Errorf("push %s: %w", r.Secret.Path, err)
		}
		fmt.Printf("  Pushed (%s): %s\n", r.Status, r.Secret.Path)
		pushed++
	}

	fmt.Printf("\nPush complete: %d pushed, %d unchanged\n", pushed, len(results)-pushed)
	return nil
}
