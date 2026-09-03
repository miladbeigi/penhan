package commands

import (
	"os"
	"testing"
)

func TestLoadSafeConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg, err := loadSafeConfig()
	if err == nil {
		t.Fatal("expected error when penhan.yaml is missing")
	}
	if cfg != nil {
		t.Fatal("expected nil config on error")
	}

	want := "no penhan.yaml in the current directory; run this command inside a safe (created with `penhan add`)"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}
