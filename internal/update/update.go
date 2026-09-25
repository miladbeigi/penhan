// Package update replaces the running penhan binary with the latest GitHub
// release, verifying the release checksum and that the new binary runs
// before swapping it in.
package update

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBase = "https://github.com"
	defaultRepo = "miladbeigi/penhan"
	binaryName  = "penhan"

	// maxDownload bounds every download; release archives are ~10 MB.
	maxDownload = 200 << 20
)

// Release is a published GitHub release.
type Release struct {
	Tag string
}

// Version returns the release version without the leading "v".
func (r *Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

// Updater fetches and installs releases. The zero value targets the real
// GitHub repository for the running OS and architecture.
//
// It uses github.com's release redirect and download URLs rather than the
// REST API: the API allows 60 unauthenticated requests per hour per IP, so
// on shared networks and CI runners updates would fail at random with 403.
type Updater struct {
	Base   string // GitHub web base URL
	Repo   string // owner/name
	OS     string
	Arch   string
	Client *http.Client
}

func (u *Updater) base() string {
	if u.Base != "" {
		return u.Base
	}
	return defaultBase
}

func (u *Updater) repo() string {
	if u.Repo != "" {
		return u.Repo
	}
	return defaultRepo
}

func (u *Updater) platform() (goos, goarch string) {
	goos, goarch = u.OS, u.Arch
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

func (u *Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 2 * time.Minute}
}

// Latest returns the newest published (non-prerelease) release, read from
// the redirect github.com/<repo>/releases/latest → .../releases/tag/<tag>.
func (u *Updater) Latest(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", u.base(), u.repo())
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", binaryName)

	// Stop at the redirect: its target names the tag.
	client := *u.client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check latest release: %w", err)
	}
	_ = resp.Body.Close()

	location := resp.Header.Get("Location")
	tag := path.Base(location)
	if resp.StatusCode < 300 || resp.StatusCode >= 400 || !strings.Contains(location, "/releases/tag/") || tag == "" {
		return nil, fmt.Errorf("check latest release: unexpected response from %s: %s", url, resp.Status)
	}
	return &Release{Tag: tag}, nil
}

func (u *Updater) downloadURL(rel *Release, asset string) string {
	return fmt.Sprintf("%s/%s/releases/download/%s/%s", u.base(), u.repo(), rel.Tag, asset)
}

// Install downloads rel for this platform and replaces the binary at exe.
func (u *Updater) Install(ctx context.Context, rel *Release, exe string) error {
	goos, goarch := u.platform()
	archiveName := fmt.Sprintf("%s_%s_%s_%s.tar.gz", binaryName, rel.Version(), goos, goarch)

	sumsBody, err := u.get(ctx, u.downloadURL(rel, "checksums.txt"), 1<<20)
	if errors.Is(err, errNotFound) {
		return fmt.Errorf("release %s has no checksums.txt; refusing to install an unverified binary", rel.Tag)
	}
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	want, err := checksumFor(sumsBody, archiveName)
	if err != nil {
		return fmt.Errorf("release %s has no build for %s/%s (%w)", rel.Tag, goos, goarch, err)
	}

	data, err := u.get(ctx, u.downloadURL(rel, archiveName), maxDownload)
	if err != nil {
		return fmt.Errorf("download %s: %w", archiveName, err)
	}
	if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s; refusing to install", archiveName)
	}

	bin, err := extractBinary(data)
	if err != nil {
		return fmt.Errorf("extract %s: %w", archiveName, err)
	}
	return replaceBinary(exe, bin)
}

var errNotFound = errors.New("not found")

func (u *Updater) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", binaryName)
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GET %s: %w", url, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", url, limit)
	}
	return body, nil
}

// checksumFor finds name's SHA-256 in a goreleaser checksums.txt.
func checksumFor(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s; refusing to install", name)
}

// extractBinary returns the penhan executable from a release tar.gz.
func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("no %s binary in archive", binaryName)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == binaryName {
			return io.ReadAll(io.LimitReader(tr, maxDownload))
		}
	}
}

// replaceBinary writes bin next to exe, checks that it runs, then renames it
// over exe. The rename is atomic, so an interrupted update never leaves a
// half-written binary behind.
func replaceBinary(exe string, bin []byte) (err error) {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".penhan-update-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("no permission to write to %s; re-run with sudo, or reinstall penhan in a directory you own", dir)
		}
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(bin); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}

	// A wrong-architecture or corrupt binary fails here, before exe is touched.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, tmpPath, "version").CombinedOutput(); err != nil {
		return fmt.Errorf("new binary does not run (%v): %s", err, strings.TrimSpace(string(out)))
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("replace %s: %w", exe, err)
	}
	return nil
}

// Executable returns the path of the running binary with symlinks resolved,
// so the real file is replaced rather than the link.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

var releaseVersion = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// IsRelease reports whether v is a plain release version like "0.5.1".
// Development builds ("dev", or git-describe output such as
// "v0.5.1-10-g75e83aa") are not.
func IsRelease(v string) bool {
	return releaseVersion.MatchString(v)
}

// Newer reports whether release version latest is newer than current. Both
// must be release versions; anything else compares as not newer.
func Newer(latest, current string) bool {
	l, c := releaseVersion.FindStringSubmatch(latest), releaseVersion.FindStringSubmatch(current)
	if l == nil || c == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		ln, _ := strconv.Atoi(l[i])
		cn, _ := strconv.Atoi(c[i])
		if ln != cn {
			return ln > cn
		}
	}
	return false
}
