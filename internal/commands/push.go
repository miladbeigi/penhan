package commands

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milad/penhan/internal/backends"
	"github.com/milad/penhan/internal/config"
	"github.com/milad/penhan/internal/crypto"
	"github.com/milad/penhan/internal/secrets"
	"github.com/milad/penhan/internal/state"
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

	// Load config
	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

	// Setup crypto provider
	var provider crypto.Provider
	switch cfg.Encryption.Method {
	case "gpg":
		provider = crypto.NewGPGProvider()
		if err := provider.Setup(cfg.Encryption.GPG.KeyPath, ""); err != nil {
			return err
		}
	case "aes":
		provider = crypto.NewAESProvider()
		if err := provider.Setup(cfg.Encryption.AES.KeyPath, ""); err != nil {
			return err
		}
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

	// Load local state
	statePath := filepath.Join(".penhan", "state.json")
	s, err := state.Load(statePath)
	if err != nil {
		s = state.NewState()
	}

	// Generate plan with empty remote state
	remoteState := state.NewState()
	plan := state.GeneratePlan(s, remoteState)

	// Show plan
	fmt.Println("Plan:")
	fmt.Printf("  + %d to add\n", plan.Add)
	fmt.Printf("  ~ %d to update\n", plan.Update)
	fmt.Printf("  - %d to delete\n", plan.Delete)

	if plan.Conflict > 0 {
		fmt.Printf("  ! %d conflicts\n", plan.Conflict)
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
	secretsDir := cfg.Secrets.Path
	err = filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" && ext != ".json" && ext != ".enc" {
			return nil
		}

		vaultPath := secrets.LocalToVault(path, secretsDir)
		vaultPath = strings.TrimSuffix(vaultPath, ".enc")

		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Decrypt if encrypted
		if strings.HasSuffix(path, ".enc") {
			decrypted, err := provider.Decrypt(data)
			if err != nil {
				return err
			}
			data = decrypted
		}

		// Push to backend
		if err := backend.Push(data, vaultPath); err != nil {
			return err
		}

		// Update state
		hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		s.UpdateHash(vaultPath, hash)
		s.MarkSynced(vaultPath)

		fmt.Printf("  Pushed: %s\n", path)
		return nil
	})

	if err != nil {
		return err
	}

	// Save state
	if err := state.Save(s, statePath); err != nil {
		return err
	}

	fmt.Println("\nPush complete")
	return nil
}
