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

	data, err := parseFile(t, filePath)
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
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

	data, err := parseFile(t, filePath)
	if err != nil {
		t.Fatalf("parseFile() error = %v", err)
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

func parseFile(t *testing.T, path string) (map[string]string, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return Parse(data, filepath.Ext(path))
}

// Secret values must reach the backend byte for byte. YAML's type resolution
// would turn "0012" into 10 and "0x1F" into 31.
func TestParseKeepsValuesAsWritten(t *testing.T) {
	yamlIn := strings.Join([]string{
		"pin: 0012",
		"hex: 0x1F",
		"ver: 1.10",
		"big: 12345678901234567890",
		"date: 2026-01-02",
		"flag: yes",
		"quoted: \"0012\"",
		"empty:",
		"tilde: ~",
		"multi: |",
		"  line1",
		"  line2",
		"",
	}, "\n")
	got, err := Parse([]byte(yamlIn), ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"pin": "0012", "hex": "0x1F", "ver": "1.10", "big": "12345678901234567890",
		"date": "2026-01-02", "flag": "yes", "quoted": "0012", "empty": "", "tilde": "",
		"multi": "line1\nline2\n",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("yaml %s = %q, want %q", k, got[k], v)
		}
	}

	got, err = Parse([]byte(`{"big": 12345678901234567890, "f": 1.10, "b": true, "n": null, "s": "0012"}`), ".json")
	if err != nil {
		t.Fatal(err)
	}
	want = map[string]string{"big": "12345678901234567890", "f": "1.10", "b": "true", "n": "", "s": "0012"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("json %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseYAMLEdgeCases(t *testing.T) {
	if m, err := Parse([]byte(""), ".yaml"); err != nil || len(m) != 0 {
		t.Errorf("empty file = %v, %v; want empty map", m, err)
	}
	if _, err := Parse([]byte("- a\n- b\n"), ".yaml"); err == nil {
		t.Error("a top-level list must be rejected")
	}
	if _, err := Parse([]byte("k: a\nk: b\n"), ".yaml"); err == nil {
		t.Error("duplicate keys must be rejected")
	}
	m, err := Parse([]byte("base: &b secret\ncopy: *b\n"), ".yaml")
	if err != nil || m["copy"] != "secret" {
		t.Errorf("alias = %v, %v; want copy=secret", m, err)
	}
}
