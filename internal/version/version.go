package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Set with -ldflags by `make build` and GoReleaser.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func init() {
	// `go install github.com/miladbeigi/penhan/cmd/penhan@v0.6.0` sets no
	// ldflags, but Go records the module version in the binary. Use it, so
	// such installs know their version (and `penhan update` works for them).
	if Version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			Version = strings.TrimPrefix(v, "v")
		}
	}
}

func String() string {
	return fmt.Sprintf("%s (commit=%s, date=%s, go=%s, os=%s, arch=%s)",
		Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
