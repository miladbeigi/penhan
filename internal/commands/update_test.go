package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/miladbeigi/penhan/internal/update"
	"github.com/miladbeigi/penhan/internal/version"
)

const newBinary = "#!/bin/sh\necho penhan 0.6.0\n"

// withFakeRelease serves v0.6.0 for this platform, points the update command
// at it, and returns the scratch file standing in for the installed binary.
func withFakeRelease(t *testing.T, current string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "penhan", Mode: 0o755, Size: int64(len(newBinary)), Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte(newBinary))
	_ = tw.Close()
	_ = gz.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	name := fmt.Sprintf("penhan_0.6.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	const prefix = "/miladbeigi/penhan/releases"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case prefix + "/latest":
			http.Redirect(w, r, prefix+"/tag/v0.6.0", http.StatusFound)
		case prefix + "/download/v0.6.0/" + name:
			_, _ = w.Write(archive)
		case prefix + "/download/v0.6.0/checksums.txt":
			_, _ = fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	exe := filepath.Join(t.TempDir(), "penhan")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	origUpdater, origExe, origVersion := newUpdater, executablePath, version.Version
	newUpdater = func() *update.Updater { return &update.Updater{Base: srv.URL, Client: srv.Client()} }
	executablePath = func() (string, error) { return exe, nil }
	version.Version = current
	t.Cleanup(func() { newUpdater, executablePath, version.Version = origUpdater, origExe, origVersion })
	return exe
}

func runUpdateCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	for _, name := range []string{"check", "force"} {
		_ = updateCmd.Flags().Set(name, "false")
	}
	if err := updateCmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	var err error
	out := captureStdout(t, func() { err = runUpdate(updateCmd, nil) })
	return out, err
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func exeContent(t *testing.T, exe string) string {
	t.Helper()
	b, _ := os.ReadFile(exe)
	return string(b)
}

func TestUpdateInstallsNewerRelease(t *testing.T) {
	exe := withFakeRelease(t, "0.5.1")
	out, err := runUpdateCmd(t)
	if err != nil {
		t.Fatalf("update error = %v\n%s", err, out)
	}
	if exeContent(t, exe) != newBinary {
		t.Error("binary was not replaced")
	}
	if !strings.Contains(out, "0.5.1 → 0.6.0") || !strings.Contains(out, "Updated") {
		t.Errorf("output = %q", out)
	}
}

func TestUpdateCheckNeverInstalls(t *testing.T) {
	exe := withFakeRelease(t, "0.5.1")
	out, err := runUpdateCmd(t, "--check")
	if err != nil {
		t.Fatal(err)
	}
	if exeContent(t, exe) != "old" {
		t.Error("--check must not install")
	}
	if !strings.Contains(out, "0.5.1 → 0.6.0") {
		t.Errorf("output = %q", out)
	}
}

func TestUpdateUpToDate(t *testing.T) {
	exe := withFakeRelease(t, "0.6.0")
	out, err := runUpdateCmd(t)
	if err != nil || !strings.Contains(out, "up to date") {
		t.Fatalf("update = %q, %v", out, err)
	}
	if exeContent(t, exe) != "old" {
		t.Error("an up-to-date binary must not be replaced")
	}
}

func TestUpdateRefusesDevBuildWithoutForce(t *testing.T) {
	exe := withFakeRelease(t, "v0.5.1-10-g75e83aa")
	if _, err := runUpdateCmd(t); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("update error = %v, want a --force hint", err)
	}
	if exeContent(t, exe) != "old" {
		t.Error("a development build must not be replaced without --force")
	}

	if _, err := runUpdateCmd(t, "--force"); err != nil {
		t.Fatalf("update --force error = %v", err)
	}
	if exeContent(t, exe) != newBinary {
		t.Error("--force should install the release")
	}
}

func TestUpdateNoticeDisabledOutsideTerminal(t *testing.T) {
	orig := version.Version
	version.Version = "0.5.1"
	t.Cleanup(func() { version.Version = orig })
	// go test's stderr is not a terminal, and scripts must never see a notice.
	if updateNoticeEnabled(checkCmd) {
		t.Error("the update notice must be off when stderr is not a terminal")
	}
	t.Setenv("PENHAN_NO_UPDATE_NOTIFIER", "1")
	if updateNoticeEnabled(checkCmd) {
		t.Error("PENHAN_NO_UPDATE_NOTIFIER must disable the notice")
	}
}
