package commands

import (
	"testing"
)

func TestPushCommandRegistration(t *testing.T) {
	// Verify push command is registered
	cmd, _, err := rootCmd.Find([]string{"push"})
	if err != nil {
		t.Fatalf("push command not found: %v", err)
	}

	if cmd.Use != "push" {
		t.Errorf("Use = %q, want %q", cmd.Use, "push")
	}
}

func TestPushCommandFlags(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"push"})
	if err != nil {
		t.Fatalf("push command not found: %v", err)
	}

	// Check force flag exists
	flag := cmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("force flag not found")
	}

	if flag.DefValue != "false" {
		t.Errorf("force flag default = %q, want %q", flag.DefValue, "false")
	}
}
