package secrets

import (
	"testing"
)

func TestLocalToVault(t *testing.T) {
	tests := []struct {
		name      string
		localPath string
		want      string
	}{
		{
			name:      "simple path",
			localPath: "secrets/db/password.yaml",
			want:      "db/password",
		},
		{
			name:      "nested path",
			localPath: "secrets/api/v1/keys.yaml",
			want:      "api/v1/keys",
		},
		{
			name:      "json file",
			localPath: "secrets/prod/cert.json",
			want:      "prod/cert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LocalToVault(tt.localPath, "secrets/")
			if got != tt.want {
				t.Errorf("LocalToVault() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVaultToLocal(t *testing.T) {
	tests := []struct {
		name      string
		vaultPath string
		want      string
	}{
		{
			name:      "simple path",
			vaultPath: "db/password",
			want:      "secrets/db/password.yaml",
		},
		{
			name:      "nested path",
			vaultPath: "api/v1/keys",
			want:      "secrets/api/v1/keys.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VaultToLocal(tt.vaultPath, "secrets/", "yaml")
			if got != tt.want {
				t.Errorf("VaultToLocal() = %q, want %q", got, tt.want)
			}
		})
	}
}
