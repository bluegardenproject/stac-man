// Package update implements the lookup and self-install logic behind
// the `sm update` command.
//
// It is intentionally tiny: it talks to the GitHub releases API to find
// the latest tag, compares it numerically (dot-split) against the
// running binary's version, and shells out to the install script to
// perform the actual replacement.
//
// All non-trivial logic (Compare) is pure so it's easy to unit-test;
// the network and shell pieces are kept thin.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repo to query for releases. Exposed as a var so
// tests can point at a stub server, but production code does not change
// it.
var Repo = "bluegardenproject/stac-man"

// InstallScriptURL is the bash one-liner target for Unix self-install.
const InstallScriptURL = "https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/install.sh"

// InstallScriptURLPS1 is the powershell self-install target.
const InstallScriptURLPS1 = "https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/install.ps1"

// Release is the slice of the GitHub releases API payload we care
// about. Anything else is intentionally dropped on the floor.
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

// LatestRelease fetches the latest release for the configured repo.
// The 5-second timeout is short enough to keep `sm update --check`
// responsive even when GitHub is slow.
func LatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Repo)

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("no published releases found")
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github api %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}

// Compare returns -1, 0, or +1 like strings.Compare, treating both
// arguments as dot-separated numeric versions. A leading "v" is
// tolerated. The literals "dev" and "unknown" are treated as the
// minimum version (any tagged release is "newer").
//
// Non-numeric segments fall back to lexicographic comparison so
// pre-release suffixes like "0.2.0-rc1" still produce a deterministic
// answer (rc1 < rc2), even if it isn't strictly SemVer-compliant.
func Compare(a, b string) int {
	if a == b {
		return 0
	}

	aDev := isDev(a)
	bDev := isDev(b)
	switch {
	case aDev && bDev:
		return 0
	case aDev:
		return -1
	case bDev:
		return 1
	}

	aParts := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bParts := strings.Split(strings.TrimPrefix(b, "v"), ".")

	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for len(aParts) < n {
		aParts = append(aParts, "0")
	}
	for len(bParts) < n {
		bParts = append(bParts, "0")
	}

	for i := 0; i < n; i++ {
		ai, aErr := strconv.Atoi(aParts[i])
		bi, bErr := strconv.Atoi(bParts[i])
		if aErr == nil && bErr == nil {
			switch {
			case ai < bi:
				return -1
			case ai > bi:
				return 1
			}
			continue
		}
		if c := strings.Compare(aParts[i], bParts[i]); c != 0 {
			return c
		}
	}
	return 0
}

func isDev(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "" || v == "dev" || v == "unknown"
}

// Run executes the install script for the current platform, streaming
// its stdout/stderr through to the caller's. It does NOT check whether
// an update is actually needed; the caller decides that.
func Run(ctx context.Context) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.CommandContext(ctx, "powershell", "-Command",
			fmt.Sprintf("iwr -useb %s | iex", InstallScriptURLPS1))
	default:
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("curl -fsSL %s | bash", InstallScriptURL))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
