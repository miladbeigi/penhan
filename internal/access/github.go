package access

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// githubURL is where SSH keys are fetched from. PENHAN_GITHUB_URL overrides
// it so tests can serve keys locally.
func githubURL() string {
	if v := os.Getenv("PENHAN_GITHUB_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://github.com"
}

var githubUser = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)

// FetchGitHubKeys returns the SSH keys user has on GitHub that age can
// encrypt to, and a note for each key it skipped.
func FetchGitHubKeys(ctx context.Context, user string) (keys []Key, skipped []string, err error) {
	if !githubUser.MatchString(user) {
		return nil, nil, fmt.Errorf("invalid GitHub username %q", user)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubURL()+"/"+user+".keys", http.NoBody)
	if err != nil {
		return nil, nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch %s's SSH keys from GitHub: %w", user, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, fmt.Errorf("GitHub user %q not found", user)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch %s's SSH keys from GitHub: %s", user, resp.Status)
	}

	sc := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		k, err := ParseKey(line)
		if err != nil {
			skipped = append(skipped, err.Error())
			continue
		}
		keys = append(keys, k)
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	if len(keys) == 0 {
		msg := fmt.Sprintf("GitHub user %q has no usable SSH keys", user)
		if len(skipped) > 0 {
			msg += ": " + strings.Join(skipped, "; ")
		}
		return nil, nil, errors.New(msg)
	}
	return keys, skipped, nil
}
