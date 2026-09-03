package prompt

import (
	"fmt"
	"regexp"

	"github.com/charmbracelet/huh"
)

type InitAnswers struct {
	SafeName       string
	Encryption     string
	GitHubUsername string
	Backend        string
	VaultAddr      string
	VaultToken     string
	RemoteDir      string
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
				huh.NewOption("GitHub GPG (seal-only)", "github-gpg"),
				huh.NewOption("AES", "aes"),
			)
		if err := encSelect.Value(&answers.Encryption).Run(); err != nil {
			return nil, err
		}
	}

	if answers.Encryption == "github-gpg" && answers.GitHubUsername == "" {
		userInput := huh.NewInput().
			Title("GitHub username").
			Placeholder("your-github-username").
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("github username is required")
				}
				return nil
			})
		if err := userInput.Value(&answers.GitHubUsername).Run(); err != nil {
			return nil, err
		}
	}

	if answers.Backend == "" {
		beSelect := huh.NewSelect[string]().
			Title("Backend type").
			Options(
				huh.NewOption("Vault", "vault"),
				huh.NewOption("File (encrypted on disk, git-committed)", "file"),
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

	return &answers, nil
}
