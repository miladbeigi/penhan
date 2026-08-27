package secrets

import (
	"path/filepath"
	"strings"
)

// LocalToVault converts a local file path to a Vault secret path.
// Example: "secrets/db/password.yaml" with secretsDir "secrets/" → "db/password"
func LocalToVault(localPath, secretsDir string) string {
	relative := strings.TrimPrefix(localPath, secretsDir)
	relative = strings.TrimPrefix(relative, "/")

	ext := filepath.Ext(relative)
	return strings.TrimSuffix(relative, ext)
}

// VaultToLocal converts a Vault secret path to a local file path.
// Example: "db/password" with secretsDir "secrets/" and format "yaml" → "secrets/db/password.yaml"
func VaultToLocal(vaultPath, secretsDir, format string) string {
	return filepath.Join(secretsDir, vaultPath+"."+format)
}
