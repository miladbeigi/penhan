package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/miladbeigi/penhan/internal/update"
	"github.com/miladbeigi/penhan/internal/version"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update penhan to the latest release",
	Long: `Update downloads the latest penhan release from GitHub for this OS and
architecture, verifies it against the release's checksums.txt, checks that
the new binary runs, and then atomically replaces the running binary.

Development builds (not installed from a release) are only replaced with
--force.`,
	Args: cobra.NoArgs,
	RunE: runUpdate,
}

// Replaced in tests: a fake release server, and a scratch file standing in
// for the running binary (which, under go test, is the test binary itself).
var (
	newUpdater     = func() *update.Updater { return &update.Updater{} }
	executablePath = update.Executable
)

func init() {
	updateCmd.Flags().Bool("check", false, "Only report whether a newer release exists")
	updateCmd.Flags().Bool("force", false, "Install the latest release even if this build is newer or a development build")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")
	current := version.Version

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	u := newUpdater()
	rel, err := u.Latest(ctx)
	if err != nil {
		return err
	}
	latest := rel.Version()

	switch {
	case update.Newer(latest, current):
		fmt.Printf("A new release is available: %s → %s\n", current, latest)
	case !update.IsRelease(current):
		fmt.Printf("This is a development build (%s); the latest release is %s.\n", current, latest)
		if !checkOnly && !force {
			return fmt.Errorf("not replacing a development build; pass --force to install %s", latest)
		}
	default:
		fmt.Printf("penhan %s is up to date.\n", current)
		if !force {
			return nil
		}
	}
	if checkOnly {
		return nil
	}

	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("locate the running binary: %w", err)
	}
	fmt.Printf("Downloading penhan %s for this platform...\n", latest)
	if err := u.Install(ctx, rel, exe); err != nil {
		return err
	}
	fmt.Printf("✓ Updated %s to %s (checksum verified)\n", exe, latest)
	return nil
}

// notifyResult carries a finished background update check.
var notifyResult chan string

// startUpdateNotice checks for a newer release in the background, at most
// once a day, when a person is at the terminal. It never installs anything.
func startUpdateNotice(cmd *cobra.Command) {
	if !updateNoticeEnabled(cmd) {
		return
	}
	cachePath, err := update.DefaultCachePath()
	if err != nil {
		return
	}
	notifyResult = make(chan string, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		n := &update.Notifier{CachePath: cachePath, Interval: 24 * time.Hour, Updater: newUpdater()}
		notifyResult <- n.NewerVersion(ctx, version.Version)
	}()
}

// printUpdateNotice prints the result of startUpdateNotice, waiting briefly
// for it so a slow network never holds up the command.
func printUpdateNotice() {
	if notifyResult == nil {
		return
	}
	select {
	case latest := <-notifyResult:
		if latest != "" {
			fmt.Fprintf(os.Stderr, "\nA new penhan release is available: %s → %s. Run `penhan update` to install it.\n", version.Version, latest)
		}
	case <-time.After(500 * time.Millisecond):
	}
}

func updateNoticeEnabled(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "update", "completion", "help", "__complete":
		return false
	}
	if os.Getenv("PENHAN_NO_UPDATE_NOTIFIER") != "" || os.Getenv("CI") != "" {
		return false
	}
	return update.IsRelease(version.Version) && term.IsTerminal(os.Stderr.Fd())
}
