package prompt

import (
	"os"
	"testing"
)

func TestInitAnswersAllPopulated(t *testing.T) {
	partial := &InitAnswers{
		Encryption: "aes",
		Backend:    "vault",
		VaultAddr:  "http://127.0.0.1:8200",
		VaultToken: "test-token",
	}
	result, err := RunInitPrompts(partial)
	if err != nil {
		t.Fatal(err)
	}
	if result.Encryption != "aes" {
		t.Errorf("Encryption = %q, want %q", result.Encryption, "aes")
	}
	if result.Backend != "vault" {
		t.Errorf("Backend = %q, want %q", result.Backend, "vault")
	}
	if result.VaultAddr != "http://127.0.0.1:8200" {
		t.Errorf("VaultAddr = %q, want %q", result.VaultAddr, "http://127.0.0.1:8200")
	}
	if result.VaultToken != "test-token" {
		t.Errorf("VaultToken = %q, want %q", result.VaultToken, "test-token")
	}
}

func TestRunRemoveSelectNonTTY(t *testing.T) {
	if info, err := os.Stdin.Stat(); err == nil && (info.Mode()&os.ModeCharDevice) == 0 {
		t.Skip("non-TTY, skipping interactive test")
	}
}
