package secrets

import (
	"path/filepath"
	"strings"
)

// IsSecretFile reports whether name (after stripping any .enc suffix) has
// an extension penhan reads as a secret: .yaml, .yml, or .json.
func IsSecretFile(name string) bool {
	switch filepath.Ext(strings.TrimSuffix(name, ".enc")) {
	case ".yaml", ".yml", ".json":
		return true
	}
	return false
}

// RemotePath converts a local secret file path to its backend path: the
// path relative to secretsDir, slash-separated, without extension.
// Example: "secrets/db/password.yaml" with secretsDir "secrets/" → "db/password"
func RemotePath(localPath, secretsDir string) string {
	rel, err := filepath.Rel(secretsDir, localPath)
	if err != nil {
		rel = localPath
	}
	rel = filepath.ToSlash(rel)
	return strings.TrimSuffix(rel, filepath.Ext(rel))
}
