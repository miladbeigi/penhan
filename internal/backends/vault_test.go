package backends

import (
	"testing"
)

func TestVaultProviderSetup(t *testing.T) {
	provider := NewVaultProvider()

	err := provider.Setup("https://vault.example.com", "test-token", "secret", "")
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if !provider.IsInitialized() {
		t.Error("IsInitialized() = false, want true")
	}
}
