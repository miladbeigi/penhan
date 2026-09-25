package prompt

import (
	"fmt"
	"regexp"

	"github.com/charmbracelet/huh"
)

type InitAnswers struct {
	SafeName   string
	Encryption string
	Backend    string
	VaultAddr  string
	VaultToken string
	RemoteDir  string

	Kubeconfig    string
	KubeContext   string
	KubeNamespace string
}

var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func ValidateSafeName(name string) error {
	if name == "" {
		return fmt.Errorf("safe name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("safe name must be 64 characters or less")
	}
	if !safeNamePattern.MatchString(name) {
		return fmt.Errorf("safe name must start with a letter or number and contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}

func RunInitPrompts(partial *InitAnswers) (*InitAnswers, error) {
	answers := *partial

	if answers.Encryption == "" {
		encSelect := huh.NewSelect[string]().
			Title("Encryption method").
			Options(
				huh.NewOption("GPG", "gpg"),
				huh.NewOption("AES", "aes"),
			)
		if err := encSelect.Value(&answers.Encryption).Run(); err != nil {
			return nil, err
		}
	}

	if answers.Backend == "" {
		beSelect := huh.NewSelect[string]().
			Title("Backend type").
			Options(
				huh.NewOption("Vault", "vault"),
				huh.NewOption("File (encrypted on disk, git-committed)", "file"),
				huh.NewOption("Kubernetes (Secrets in a cluster namespace)", "kubernetes"),
			)
		if err := beSelect.Value(&answers.Backend).Run(); err != nil {
			return nil, err
		}
	}

	if answers.Backend == "vault" {
		if answers.VaultAddr == "" {
			addrInput := huh.NewInput().
				Title("Vault address").
				Placeholder("http://127.0.0.1:8200").
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("vault address is required")
					}
					return nil
				})
			if err := addrInput.Value(&answers.VaultAddr).Run(); err != nil {
				return nil, err
			}
		}

		if answers.VaultToken == "" {
			tokenInput := huh.NewInput().
				Title("Vault token").
				EchoMode(huh.EchoModePassword).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("vault token is required")
					}
					return nil
				})
			if err := tokenInput.Value(&answers.VaultToken).Run(); err != nil {
				return nil, err
			}
		}
	}

	if answers.Backend == "kubernetes" && answers.KubeNamespace == "" {
		nsInput := huh.NewInput().
			Title("Kubernetes namespace").
			Placeholder("default").
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("namespace is required")
				}
				return nil
			})
		if err := nsInput.Value(&answers.KubeNamespace).Run(); err != nil {
			return nil, err
		}
	}

	return &answers, nil
}

// SelectKubeContext asks which kubeconfig context the safe should push to,
// with current preselected.
func SelectKubeContext(contexts []string, current string) (string, error) {
	choice := current
	options := make([]huh.Option[string], 0, len(contexts))
	for _, c := range contexts {
		options = append(options, huh.NewOption(c, c))
	}
	sel := huh.NewSelect[string]().
		Title("Kubernetes context").
		Description("Recorded in penhan.yaml, so pushes always target this cluster").
		Options(options...)
	if err := sel.Value(&choice).Run(); err != nil {
		return "", err
	}
	return choice, nil
}
