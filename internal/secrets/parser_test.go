package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAMLFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "secret.yaml")

	content := `
username: admin
password: s3cret
api_key: abc123
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if data["username"] != "admin" {
		t.Errorf("data[username] = %q, want %q", data["username"], "admin")
	}

	if data["password"] != "s3cret" {
		t.Errorf("data[password] = %q, want %q", data["password"], "s3cret")
	}
}

func TestParseJSONFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "secret.json")

	content := `{"username": "admin", "password": "s3cret"}`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	data, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if data["username"] != "admin" {
		t.Errorf("data[username] = %q, want %q", data["username"], "admin")
	}
}

func TestWriteParseRoundtrip(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "secret.yaml")

	original := map[string]string{
		"username": "admin",
		"password": "s3cret",
	}

	if err := WriteFile(filePath, original); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	parsed, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	for k, v := range original {
		if parsed[k] != v {
			t.Errorf("parsed[%s] = %q, want %q", k, parsed[k], v)
		}
	}
}
