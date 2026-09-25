package commands

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/backends"
	"github.com/miladbeigi/penhan/internal/secrets"
)

// Every value a Secret can hold as text must survive the trip into a YAML
// secret file unchanged, or check would report it changed forever (and push
// would corrupt it).
func TestSecretYAMLRoundtripsTrickyValues(t *testing.T) {
	values := map[string]string{
		"octal":        "0012",
		"hex":          "0x1F",
		"float":        "1.10",
		"bool":         "yes",
		"null":         "null",
		"tilde":        "~",
		"empty":        "",
		"date":         "2026-01-02",
		"spaces":       "  padded  ",
		"multiline":    "line1\nline2\n",
		"noeol":        "line1\nline2",
		"json":         `{"AccountTag":"abc","TunnelSecret":"x=="}`,
		"colon":        "a: b",
		"hash":         "value # not a comment",
		"quotes":       `it's "quoted"`,
		"unicode":      "こんにちは",
		"tab":          "a\tb",
		"leading-dash": "- item",
		"star":         "*alias",
	}
	content, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	out, err := secretYAML(content)
	if err != nil {
		t.Fatalf("secretYAML() error = %v", err)
	}
	parsed, err := secrets.Parse(out, ".yaml")
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range values {
		if parsed[k] != want {
			t.Errorf("%s = %q, want %q\nfile:\n%s", k, parsed[k], want, out)
		}
	}
}

// fakeImporter serves one secret and can fail adoption.
type fakeImporter struct {
	content  []byte
	adoptErr error
	adopted  []string
}

func (f *fakeImporter) ImportCandidates() ([]backends.ImportCandidate, error) { return nil, nil }
func (f *fakeImporter) ReadForImport(name string) (path string, content []byte, err error) {
	return name, f.content, nil
}
func (f *fakeImporter) Adopt(path string, _ []byte) error {
	if f.adoptErr != nil {
		return f.adoptErr
	}
	f.adopted = append(f.adopted, path)
	return nil
}

func TestImportSecretWritesOnlyEncryptedFile(t *testing.T) {
	cfg, provider := newTestSafe(t)
	imp := &fakeImporter{content: []byte(`{"API_KEY":"s3cret-0012"}`)}

	if err := importSecret(cfg, provider, imp, "api"); err != nil {
		t.Fatal(err)
	}
	enc, err := os.ReadFile(filepath.Join("secrets", "api.yaml.enc"))
	if err != nil {
		t.Fatalf("expected secrets/api.yaml.enc: %v", err)
	}
	if strings.Contains(string(enc), "s3cret") {
		t.Error("the imported file must be encrypted")
	}
	if _, err := os.Stat(filepath.Join("secrets", "api.yaml")); !os.IsNotExist(err) {
		t.Error("import must never write the plaintext")
	}
	if len(imp.adopted) != 1 || imp.adopted[0] != "api" {
		t.Errorf("adopted = %v", imp.adopted)
	}

	// The imported file reads back as the same content, so check sees no change.
	local, err := collectLocalSecrets(cfg, provider)
	if err != nil || len(local) != 1 || string(local[0].Content) != `{"API_KEY":"s3cret-0012"}` {
		t.Fatalf("local secrets = %+v, %v", local, err)
	}
}

func TestImportSecretRemovesFileWhenAdoptFails(t *testing.T) {
	cfg, provider := newTestSafe(t)
	imp := &fakeImporter{content: []byte(`{"k":"v"}`), adoptErr: errors.New("changed during import")}

	if err := importSecret(cfg, provider, imp, "db"); err == nil {
		t.Fatal("expected the adopt error")
	}
	if _, err := os.Stat(filepath.Join("secrets", "db.yaml.enc")); !os.IsNotExist(err) {
		t.Error("a failed import must not leave a file behind")
	}
}

func TestImportSecretRefusesExistingLocalFile(t *testing.T) {
	cfg, provider := newTestSafe(t)
	writeSecretFile(t, "db.json", `{"k":"local"}`)
	imp := &fakeImporter{content: []byte(`{"k":"v"}`)}

	if err := importSecret(cfg, provider, imp, "db"); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("importSecret() error = %v, want already exists", err)
	}
	if len(imp.adopted) != 0 {
		t.Error("nothing may be adopted when the local file exists")
	}
}
