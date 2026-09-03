//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReportsNewSecret(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "fresh.yaml", "v: 1\n")

	out := mustCheck(t, s)
	if !strings.Contains(out, "new        fresh") {
		t.Errorf("check should report fresh as new:\n%s", out)
	}
	if !strings.Contains(out, "1 secret(s), 1 to push") {
		t.Errorf("check summary wrong:\n%s", out)
	}
	if data := readVaultData(t, s, "fresh"); data != nil {
		t.Errorf("check must not write to vault, found %v", data)
	}
}

func TestCheckStatusesAfterPush(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "same.yaml", "v: 1\n")
	writeSecret(t, s, "edited.yaml", "v: 1\n")
	mustPush(t, s)

	writeSecret(t, s, "edited.yaml", "v: 2\n")
	writeSecret(t, s, "added.yaml", "v: 1\n")

	out := mustCheck(t, s)
	for _, want := range []string{"unchanged  same", "changed    edited", "new        added", "3 secret(s), 2 to push"} {
		if !strings.Contains(out, want) {
			t.Errorf("check output missing %q:\n%s", want, out)
		}
	}

	if data := readVaultData(t, s, "edited"); data["v"] != "1" {
		t.Errorf("check must not push the edit, vault has %v", data)
	}
	if data := readVaultData(t, s, "added"); data != nil {
		t.Errorf("check must not push the new secret, vault has %v", data)
	}
}

func TestCheckOnlyCoversLocalFiles(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "mine.yaml", "v: 1\n")
	mustPush(t, s)
	vaultPut(t, s, "remote-only", "v=x")

	out := mustCheck(t, s)
	if strings.Contains(out, "remote-only") {
		t.Errorf("check must not report secrets that exist only in the backend:\n%s", out)
	}
	if !strings.Contains(out, "1 secret(s), 0 to push") {
		t.Errorf("check summary wrong:\n%s", out)
	}
}

func TestCheckEmptySafe(t *testing.T) {
	s := newVaultSafe(t)
	out := mustCheck(t, s)
	if !strings.Contains(out, "No secrets found") {
		t.Errorf("empty safe should say so:\n%s", out)
	}
}

func TestCheckFailsWhenBackendUnreachable(t *testing.T) {
	s := newVaultSafe(t)
	writeSecret(t, s, "offline.yaml", "v: 1\n")

	cfgPath := filepath.Join(s.Dir, "penhan.yaml")
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(cfg), vaultAddr, "http://127.0.0.1:1", 1)
	if err := os.WriteFile(cfgPath, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runPenhan(t, s.Dir, "check")
	if code == 0 {
		t.Fatalf("check must fail when the backend cannot be reached: %s", stdout)
	}
	if !strings.Contains(stderr, "offline") {
		t.Errorf("error should name the secret it could not read: %s", stderr)
	}
}
