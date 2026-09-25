package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Notifier looks for a newer release at most once per Interval and caches
// the answer, so penhan contacts GitHub at most once a day.
type Notifier struct {
	CachePath string
	Interval  time.Duration
	Updater   *Updater
	Now       func() time.Time
}

type notifyCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// DefaultCachePath is where the last check result is stored.
func DefaultCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "penhan", "update-check.json"), nil
}

// NewerVersion returns the latest release version if it is newer than
// current, or "" otherwise. Network and cache errors are swallowed: a
// failed check must never get in the way of the command being run.
func (n *Notifier) NewerVersion(ctx context.Context, current string) string {
	now := time.Now
	if n.Now != nil {
		now = n.Now
	}

	var c notifyCache
	if data, err := os.ReadFile(n.CachePath); err == nil {
		_ = json.Unmarshal(data, &c)
	}

	if now().Sub(c.CheckedAt) >= n.Interval {
		// Record the attempt even when it fails, so an offline machine
		// does not retry on every command.
		c.CheckedAt = now()
		if rel, err := n.Updater.Latest(ctx); err == nil {
			c.Latest = rel.Version()
		}
		if data, err := json.Marshal(c); err == nil {
			_ = os.MkdirAll(filepath.Dir(n.CachePath), 0o700)
			_ = os.WriteFile(n.CachePath, data, 0o600)
		}
	}

	if Newer(c.Latest, current) {
		return c.Latest
	}
	return ""
}
