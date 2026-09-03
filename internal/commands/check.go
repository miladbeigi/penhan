package commands

import (
	"fmt"

	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/config"
	"github.com/miladbeigi/penhan/internal/crypto"
	"github.com/spf13/cobra"
)

// Check statuses for a local secret relative to the backend.
const (
	StatusNew       = "new"       // no secret at this path in the backend
	StatusChanged   = "changed"   // backend holds different content
	StatusUnchanged = "unchanged" // backend content hashes the same
)

// checkResult is the outcome of comparing one local secret with the backend.
type checkResult struct {
	Secret localSecret
	Status string
}

// NeedsPush reports whether push should write this secret.
func (r checkResult) NeedsPush() bool {
	return r.Status != StatusUnchanged
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Compare local secrets with the backend without changing anything",
	Long: `Check hashes every secret file in the secrets directory and compares it with
what the backend holds at the same path. It reports each secret as new,
changed, or unchanged. Nothing is written locally or remotely.

Only secrets that exist locally are checked; secrets that live solely in the
backend are not reported.`,
	Args: cobra.NoArgs,
	RunE: runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
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

	printCheck(results)
	return nil
}

// checkSecrets compares every local secret against the backend.
func checkSecrets(cfg *config.Config, provider crypto.Provider, backend backends.Provider) ([]checkResult, error) {
	local, err := collectLocalSecrets(cfg, provider)
	if err != nil {
		return nil, err
	}

	results := make([]checkResult, 0, len(local))
	for _, s := range local {
		remote, err := remoteHash(backend, s.Path)
		if err != nil {
			return nil, err
		}

		status := StatusUnchanged
		switch {
		case remote == "":
			status = StatusNew
		case remote != s.Hash:
			status = StatusChanged
		}
		results = append(results, checkResult{Secret: s, Status: status})
	}

	return results, nil
}

func printCheck(results []checkResult) {
	if len(results) == 0 {
		fmt.Println("No secrets found.")
		return
	}

	pending := 0
	for _, r := range results {
		fmt.Printf("  %-10s %s\n", r.Status, r.Secret.Path)
		if r.NeedsPush() {
			pending++
		}
	}

	fmt.Printf("\n%d secret(s), %d to push\n", len(results), pending)
}
