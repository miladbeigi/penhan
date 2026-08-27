package commands

import (
	"fmt"
	"path/filepath"

	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/state"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show what push/pull would do (dry-run)",
	RunE:  runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	provider, err := newCryptoProvider(cfg)
	if err != nil {
		return err
	}

	statePath := filepath.Join(".penhan", "state.json")
	s, err := loadStateOrNew(statePath)
	if err != nil {
		return err
	}

	local, err := collectLocalSecrets(cfg, provider)
	if err != nil {
		return err
	}

	// Fetch remote state from Vault when reachable; plan still works offline.
	remoteState := state.NewState()
	if backend, err := newVaultBackend(cfg); err == nil {
		if rs, err := buildRemoteState(s, backend); err == nil {
			remoteState = rs
		}
	}

	plan := state.GeneratePlan(buildLocalState(s, local), remoteState)

	// Show plan
	fmt.Println("Plan:")
	fmt.Printf("  + %d to add\n", plan.Add)
	fmt.Printf("  ~ %d to update\n", plan.Update)
	fmt.Printf("  - %d to delete\n", plan.Delete)

	if plan.Conflict > 0 {
		fmt.Printf("  ! %d conflicts\n", plan.Conflict)
	}

	fmt.Println()
	for _, change := range plan.Changes {
		switch change.Action {
		case "add":
			fmt.Printf("  + %s (new)\n", change.Path)
		case "update":
			fmt.Printf("  ~ %s (changed)\n", change.Path)
		case "delete":
			fmt.Printf("  - %s (deleted)\n", change.Path)
		case "conflict":
			fmt.Printf("  ! %s (CONFLICT)\n", change.Path)
		}
	}

	return nil
}
