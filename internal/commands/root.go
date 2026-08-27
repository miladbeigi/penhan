package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "penhan",
	Short: "Manage secrets in Git with encryption",
	Long:  `Penhan is a CLI tool for managing secrets securely using Git as the source of truth.`,
	// Runtime errors are reported once by Execute; dumping the usage block
	// on top of them buries the actual message.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
