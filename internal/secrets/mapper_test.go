package secrets

import (
	"testing"
)

func TestRemotePath(t *testing.T) {
	tests := []struct {
		name       string
		localPath  string
		secretsDir string
		want       string
	}{
		{"simple path", "secrets/db/password.yaml", "secrets/", "db/password"},
		{"nested path", "secrets/api/v1/keys.yaml", "secrets/", "api/v1/keys"},
		{"json file", "secrets/prod/cert.json", "secrets/", "prod/cert"},
		{"no trailing slash", "secrets/db.yaml", "secrets", "db"},
		// filepath.Walk yields cleaned paths, so "./secrets" must still match.
		{"dot-prefixed dir", "secrets/db.yaml", "./secrets", "db"},
		{"dot in name", "secrets/db.prod.yaml", "secrets/", "db.prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RemotePath(tt.localPath, tt.secretsDir); got != tt.want {
				t.Errorf("RemotePath(%q, %q) = %q, want %q", tt.localPath, tt.secretsDir, got, tt.want)
			}
		})
	}
}

func TestIsSecretFile(t *testing.T) {
	for name, want := range map[string]bool{
		"db.yaml":      true,
		"db.yml":       true,
		"db.json":      true,
		"db.yaml.enc":  true,
		"db.json.enc":  true,
		".gitkeep":     false,
		"README.md":    false,
		"db.enc":       false,
		"db.yaml.orig": false,
	} {
		if got := IsSecretFile(name); got != want {
			t.Errorf("IsSecretFile(%q) = %v, want %v", name, got, want)
		}
	}
}
