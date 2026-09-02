package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/state"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Encrypt and sync local secrets to Vault",
	RunE:  runPush,
}

func init() {
	pushCmd.Flags().Bool("force", false, "Force push even with conflicts")
	rootCmd.AddCommand(pushCmd)
}

func runPush(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	provider, err := newCryptoProvider(cfg)
	if err != nil {
		return err
	}

	backend, err := newVaultBackend(cfg)
	if err != nil {
		return err
	}

	statePath := filepath.Join(".penhan", "state.json")
	s, err := loadStateOrNew(statePath)
	if err != nil {
		return err
	}

	// Compare current local files against the backend to detect conflicts:
	// a secret changed both locally and remotely since the last sync.
	local, err := collectLocalSecrets(cfg, provider)
	if err != nil {
		return err
	}
	remoteState, err := buildRemoteState(s, backend)
	if err != nil {
		return err
	}
	plan := state.GeneratePlan(buildLocalState(s, local), remoteState)

	// Show plan
	fmt.Println("Plan:")
	fmt.Printf("  + %d to add\n", plan.Add)
	fmt.Printf("  ~ %d to update\n", plan.Update)
	fmt.Printf("  - %d to delete\n", plan.Delete)

	if plan.Conflict > 0 {
		fmt.Printf("  ! %d conflicts\n", plan.Conflict)
		for _, change := range plan.Changes {
			if change.Action == "conflict" {
				fmt.Printf("  ! %s (CONFLICT)\n", change.Path)
			}
		}
		if !force {
			fmt.Println("\nUse --force to override conflicts")
			return fmt.Errorf("conflicts detected")
		}
	}

	// Confirm
	fmt.Print("\nApply changes? (y/N) ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer != "y" && answer != "Y" {
		fmt.Println("Aborted")
		return nil
	}

	// Push secrets
	for vaultPath, content := range local {
		if err := backend.Push(content, vaultPath); err != nil {
			return err
		}
		s.SetSynced(vaultPath, hashContent(content))
		fmt.Printf("  Pushed: %s\n", vaultPath)
	}

	if err := state.Save(s, statePath); err != nil {
		return err
	}

	fmt.Println("\nPush complete")
	return nil
}
