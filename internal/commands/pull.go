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

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch secrets from Vault and decrypt locally",
	RunE:  runPull,
}

func init() {
	pullCmd.Flags().Bool("force", false, "Force pull even with conflicts")
	rootCmd.AddCommand(pullCmd)
}

func runPull(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := config.Load("penhan.yaml")
	if err != nil {
		return err
	}

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

	backend := backends.NewVaultProvider()
	token, err := os.ReadFile(cfg.Backend.Vault.TokenPath)
	if err != nil {
		return err
	}
	if err := backend.Setup(cfg.Backend.Vault.Addr, string(token), cfg.Backend.Vault.MountPath, cfg.Backend.Vault.BasePath); err != nil {
		return err
	}

	statePath := filepath.Join(".penhan", "state.json")
	s, err := state.Load(statePath)
	if err != nil {
		s = state.NewState()
	}

	plan := state.GeneratePlan(state.NewState(), s)

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

	fmt.Print("\nApply changes? (y/N) ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	if answer != "y" && answer != "Y" {
		fmt.Println("Aborted")
		return nil
	}

	secretsDir := cfg.Secrets.Path
	remoteSecrets, err := backend.List("")
	if err != nil {
		return err
	}

	for _, remotePath := range remoteSecrets {
		localPath := secrets.VaultToLocal(remotePath, secretsDir, cfg.Secrets.Format)

		data, err := backend.Pull(remotePath)
		if err != nil {
			return err
		}

		encrypted, err := provider.Encrypt(data)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(localPath+".enc"), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(localPath+".enc", encrypted, 0644); err != nil {
			return err
		}

		hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		s.UpdateRemoteHash(remotePath, hash)
		s.MarkSynced(remotePath)

		fmt.Printf("  Pulled: %s\n", localPath)
	}

	if err := state.Save(s, statePath); err != nil {
		return err
	}

	fmt.Println("\nPull complete")
	return nil
}
