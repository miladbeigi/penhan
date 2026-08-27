package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/milad/penhan/internal/backends"
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
	// Load config
	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	// Setup backend
	backend := backends.NewVaultProvider()
	token, err := os.ReadFile(cfg.Backend.Vault.TokenPath)
	if err != nil {
		return err
	}
	if err := backend.Setup(cfg.Backend.Vault.Addr, string(token), cfg.Backend.Vault.MountPath, cfg.Backend.Vault.BasePath); err != nil {
		return err
	}

	// Load state
	statePath := filepath.Join(".penhan", "state.json")
	s, err := state.Load(statePath)
	if err != nil {
		s = state.NewState()
	}

	// Generate plan
	plan := state.GeneratePlan(s, state.NewState())

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
