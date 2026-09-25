package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRelease serves a GitHub release API for one version, with a tar.gz
// holding a shell script as the "penhan" binary. Fields can be tweaked
// before calling start to simulate broken releases.
type fakeRelease struct {
	version    string
	binary     string // content of the penhan file inside the archive
	checksum   string // override for the archive's checksum ("" = correct)
	noChecksum bool
	platform   string // os_arch of the published archive
	apiHits    atomic.Int32
}

// newFakeRelease returns a well-formed v0.6.0 release for linux/amd64.
func newFakeRelease() *fakeRelease {
	return &fakeRelease{
		version:  "0.6.0",
		binary:   "#!/bin/sh\necho penhan 0.6.0\n",
		platform: "linux_amd64",
	}
}

func (f *fakeRelease) archiveName() string {
	return fmt.Sprintf("penhan_%s_%s.tar.gz", f.version, f.platform)
}

func (f *fakeRelease) start(t *testing.T) *Updater {
	t.Helper()
	archive := tarGz(t, map[string]string{"README.md": "docs", "penhan": f.binary})
	sum := sha256.Sum256(archive)
	checksum := hex.EncodeToString(sum[:])
	if f.checksum != "" {
		checksum = f.checksum
	}

	const prefix = "/miladbeigi/penhan/releases"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case prefix + "/latest":
			f.apiHits.Add(1)
			http.Redirect(w, r, prefix+"/tag/v"+f.version, http.StatusFound)
		case prefix + "/download/v" + f.version + "/" + f.archiveName():
			_, _ = w.Write(archive)
		case prefix + "/download/v" + f.version + "/checksums.txt":
			if f.noChecksum {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprintf(w, "deadbeef  penhan_%s_darwin_arm64.tar.gz\n%s  %s\n", f.version, checksum, f.archiveName())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return &Updater{Base: srv.URL, OS: "linux", Arch: "amd64", Client: srv.Client()}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeExe is a stand-in for the installed binary.
func fakeExe(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "penhan")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe
}

func install(t *testing.T, f *fakeRelease, exe string) error {
	t.Helper()
	u := f.start(t)
	rel, err := u.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return u.Install(context.Background(), rel, exe)
}

func requireUntouched(t *testing.T, exe string) {
	t.Helper()
	if got, _ := os.ReadFile(exe); string(got) != "#!/bin/sh\necho old\n" {
		t.Errorf("binary was replaced: %q", got)
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestInstallReplacesBinary(t *testing.T) {
	f := newFakeRelease()
	exe := fakeExe(t)
	if err := install(t, f, exe); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != f.binary {
		t.Errorf("binary = %q, want the release binary", got)
	}
	info, _ := os.Stat(exe)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 755", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(exe))
	if len(entries) != 1 {
		t.Errorf("temporary files left behind: %v", entries)
	}
}

func TestInstallRefusesChecksumMismatch(t *testing.T) {
	f := newFakeRelease()
	f.checksum = strings.Repeat("0", 64)
	exe := fakeExe(t)
	if err := install(t, f, exe); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Install() error = %v, want checksum mismatch", err)
	}
	requireUntouched(t, exe)
}

func TestInstallRefusesMissingChecksums(t *testing.T) {
	f := newFakeRelease()
	f.noChecksum = true
	exe := fakeExe(t)
	if err := install(t, f, exe); err == nil || !strings.Contains(err.Error(), "unverified") {
		t.Fatalf("Install() error = %v, want refusal", err)
	}
	requireUntouched(t, exe)
}

func TestInstallRefusesMissingPlatform(t *testing.T) {
	f := newFakeRelease()
	f.platform = "linux_arm64"
	exe := fakeExe(t)
	if err := install(t, f, exe); err == nil || !strings.Contains(err.Error(), "no build for linux/amd64") {
		t.Fatalf("Install() error = %v, want no build", err)
	}
	requireUntouched(t, exe)
}

// A binary that cannot run here (wrong architecture, corrupt) must never
// replace a working one.
func TestInstallRefusesBinaryThatDoesNotRun(t *testing.T) {
	f := newFakeRelease()
	f.binary = "#!/bin/sh\necho 'exec format error' >&2\nexit 126\n"
	exe := fakeExe(t)
	if err := install(t, f, exe); err == nil || !strings.Contains(err.Error(), "does not run") {
		t.Fatalf("Install() error = %v, want does not run", err)
	}
	requireUntouched(t, exe)
}

func TestNewerAndIsRelease(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.6.0", "0.5.1", true},
		{"0.5.10", "0.5.9", true},
		{"1.0.0", "0.99.99", true},
		{"0.5.1", "0.5.1", false},
		{"0.5.0", "0.5.1", false},
		{"v0.6.0", "0.5.1", true},
		{"0.6.0", "dev", false},
		{"0.6.0", "v0.5.1-10-g75e83aa", false},
		{"", "0.5.1", false},
	}
	for _, tc := range cases {
		if got := Newer(tc.latest, tc.current); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
	for v, want := range map[string]bool{"0.5.1": true, "v1.2.3": true, "dev": false, "v0.5.1-10-g75e83aa": false, "v0.5.1-dirty": false} {
		if IsRelease(v) != want {
			t.Errorf("IsRelease(%q) = %v, want %v", v, !want, want)
		}
	}
}

func TestNotifierChecksAtMostOncePerInterval(t *testing.T) {
	f := newFakeRelease()
	u := f.start(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	n := &Notifier{
		CachePath: filepath.Join(t.TempDir(), "penhan", "update-check.json"),
		Interval:  24 * time.Hour,
		Updater:   u,
		Now:       func() time.Time { return now },
	}

	if got := n.NewerVersion(context.Background(), "0.5.1"); got != "0.6.0" {
		t.Fatalf("NewerVersion() = %q, want 0.6.0", got)
	}
	now = now.Add(23 * time.Hour)
	if got := n.NewerVersion(context.Background(), "0.5.1"); got != "0.6.0" {
		t.Fatalf("cached NewerVersion() = %q, want 0.6.0", got)
	}
	if hits := f.apiHits.Load(); hits != 1 {
		t.Errorf("API hit %d times within the interval, want 1", hits)
	}
	now = now.Add(2 * time.Hour)
	n.NewerVersion(context.Background(), "0.5.1")
	if hits := f.apiHits.Load(); hits != 2 {
		t.Errorf("API hit %d times after the interval, want 2", hits)
	}
	if got := n.NewerVersion(context.Background(), "0.6.0"); got != "" {
		t.Errorf("NewerVersion() when up to date = %q, want empty", got)
	}
}

// An offline machine must not retry (and wait on the network) every command.
func TestNotifierRecordsFailedChecks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	var hits atomic.Int32
	u := &Updater{Base: srv.URL, Client: &http.Client{Transport: countingTransport{&hits, http.DefaultTransport}}}
	n := &Notifier{CachePath: filepath.Join(t.TempDir(), "c.json"), Interval: time.Hour, Updater: u}

	for i := 0; i < 3; i++ {
		if got := n.NewerVersion(context.Background(), "0.5.1"); got != "" {
			t.Fatalf("NewerVersion() = %q on failure, want empty", got)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("failed check retried %d times, want 1 attempt per interval", hits.Load())
	}
}

type countingTransport struct {
	n  *atomic.Int32
	rt http.RoundTripper
}

func (c countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.n.Add(1)
	return c.rt.RoundTrip(r)
}
