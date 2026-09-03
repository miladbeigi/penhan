package secrets

import (
	"os"
	"path/filepath"
	"strings"
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

func TestParseRejectsNestedYAMLValues(t *testing.T) {
	_, err := Parse([]byte("db:\n  host: localhost\n  port: 5432\n"), ".yaml")
	if err == nil {
		t.Fatal("Parse() = nil error, want error for nested values")
	}
	if !strings.Contains(err.Error(), "nested") {
		t.Errorf("error %q should mention nested values", err)
	}
}

func TestParseRejectsNestedJSONValues(t *testing.T) {
	_, err := Parse([]byte(`{"db": {"host": "localhost"}}`), ".json")
	if err == nil {
		t.Fatal("Parse() = nil error, want error for nested values")
	}
}

func TestParseRejectsListValues(t *testing.T) {
	_, err := Parse([]byte("hosts:\n  - a\n  - b\n"), ".yaml")
	if err == nil {
		t.Fatal("Parse() = nil error, want error for list values")
	}
}

// Scalar non-string values (ints, bools) remain fine as strings.
func TestParseAllowsScalarValues(t *testing.T) {
	m, err := Parse([]byte("port: 5432\nssl: true\n"), ".yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if m["port"] != "5432" || m["ssl"] != "true" {
		t.Errorf("got %v, want port=5432 ssl=true", m)
	}
}
