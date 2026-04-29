package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckRollup values are defined in merge.go: ChecksPass, ChecksFail,
// ChecksPending, ChecksNone. PRStatus reuses that vocabulary so the
// `sm land` gating logic and the `sm log` CI dot share one source of
// truth.

// Mergeability captures GitHub's `mergeable` field. We only care about
// the three states the user can act on; everything else collapses to
// MergeUnknown.
type Mergeability string

const (
	MergeUnknown     Mergeability = "UNKNOWN"
	MergeMergeable   Mergeability = "MERGEABLE"
	MergeConflicting Mergeability = "CONFLICTING"
)

// PRStatus is the slice of `gh pr view` data `sm log` and `sm doctor`
// need to colour rows. One round-trip per PR fills both the CI rollup
// and the mergeability; the cache below keeps reads cheap.
type PRStatus struct {
	Number    int          `json:"number"`
	Checks    CheckRollup  `json:"checks"`
	Mergeable Mergeability `json:"mergeable"`
	State     PRState      `json:"state"`
	IsDraft   bool         `json:"is_draft"`
}

// PRStatusForNumber issues a single `gh pr view` call combining the
// fields the CI dot and the mergeability glyph need. Combining the
// round-trip is the whole point of the shared status struct: each
// `sm log` invocation makes at most one gh call per PR.
func (c *Client) PRStatusForNumber(ctx context.Context, number int) (PRStatus, error) {
	out, _, err := c.r.Run(ctx, "pr", "view", fmt.Sprintf("%d", number),
		"--json", "number,mergeable,state,isDraft,statusCheckRollup")
	if err != nil {
		return PRStatus{}, err
	}
	var raw struct {
		Number            int    `json:"number"`
		Mergeable         string `json:"mergeable"`
		State             string `json:"state"`
		IsDraft           bool   `json:"isDraft"`
		StatusCheckRollup []struct {
			State      string `json:"state"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return PRStatus{}, fmt.Errorf("parsing gh pr view: %w", err)
	}
	rollupEntries := make([]checkEntry, len(raw.StatusCheckRollup))
	for i, e := range raw.StatusCheckRollup {
		rollupEntries[i] = checkEntry{State: e.State, Status: e.Status, Conclusion: e.Conclusion}
	}
	return PRStatus{
		Number:    raw.Number,
		Checks:    rollupFromEntries(rollupEntries),
		Mergeable: parseMergeable(raw.Mergeable),
		State:     PRState(strings.ToUpper(raw.State)),
		IsDraft:   raw.IsDraft,
	}, nil
}

// checkEntry mirrors the fields gh exposes on each statusCheckRollup
// element. Pulled out as a named type so rollupFromEntries can be
// unit-tested without mocking the JSON shape.
type checkEntry struct {
	State      string
	Status     string
	Conclusion string
}

// rollupFromEntries reduces a heterogeneous list of check / status
// entries to one CheckRollup value. The rules deliberately bias
// toward "tell the user something is wrong":
//
//   - any failing entry → ChecksFail
//   - else any in-flight entry → ChecksPending
//   - else ChecksPass (every entry ran and finished cleanly)
//   - empty list → ChecksNone (no checks configured at all)
//
// Conclusion takes precedence over state when both are set: gh
// reports state=COMPLETED conclusion=SUCCESS for a green check run,
// but the legacy commit-status API only sets `state` (no conclusion),
// so we have to consult both.
func rollupFromEntries(entries []checkEntry) CheckRollup {
	if len(entries) == 0 {
		return ChecksNone
	}
	anyFailing := false
	anyPending := false
	for _, e := range entries {
		conclusion := strings.ToUpper(e.Conclusion)
		state := strings.ToUpper(e.State)
		status := strings.ToUpper(e.Status)
		switch conclusion {
		case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
			anyFailing = true
			continue
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			continue
		}
		// No usable conclusion — fall back to state/status.
		switch state {
		case "FAILURE", "ERROR":
			anyFailing = true
		case "SUCCESS":
			// passing
		case "PENDING", "EXPECTED", "IN_PROGRESS", "QUEUED", "WAITING", "REQUESTED":
			anyPending = true
		default:
			// Status field is the modern check-run status enum.
			switch status {
			case "QUEUED", "IN_PROGRESS", "PENDING", "WAITING", "REQUESTED":
				anyPending = true
			case "COMPLETED":
				// COMPLETED with no conclusion is unusual; treat as
				// pending so the user investigates rather than seeing
				// a misleading green dot.
				anyPending = true
			default:
				anyPending = true
			}
		}
	}
	if anyFailing {
		return ChecksFail
	}
	if anyPending {
		return ChecksPending
	}
	return ChecksPass
}

func parseMergeable(s string) Mergeability {
	switch strings.ToUpper(s) {
	case "MERGEABLE":
		return MergeMergeable
	case "CONFLICTING":
		return MergeConflicting
	default:
		return MergeUnknown
	}
}

// checksCacheTTL is how long a fetched PRStatus stays valid before
// the next `sm log` reissues the gh round-trip. 60s is the value the
// roadmap calls out — long enough to make rapid `sm log` calls
// snappy, short enough that a CI flip is visible within a coffee
// break.
const checksCacheTTL = 60 * time.Second

// ChecksCache is the on-disk shared cache backing both the CI dot
// and the mergeability glyph. Keyed by PR number stringified so the
// JSON file stays human-readable; the entries embed PRStatus
// directly (it's already a JSON-friendly shape).
type ChecksCache struct {
	Entries map[string]checksCacheEntry `json:"entries"`
}

type checksCacheEntry struct {
	Status    PRStatus  `json:"status"`
	FetchedAt time.Time `json:"fetched_at"`
}

// ChecksCachePath returns the canonical cache file location given the
// .git directory. Callers pass the path returned by `git rev-parse
// --git-dir` so worktrees get their own cache.
func ChecksCachePath(gitDir string) string {
	return filepath.Join(gitDir, "stac-man", "checks-cache.json")
}

// LoadChecksCache reads the cache from disk, returning an empty cache
// (never nil) when the file is missing or corrupt. A corrupt cache is
// silently dropped instead of bubbling up — `sm log` should never
// fail just because the cache file is malformed.
func LoadChecksCache(gitDir string) *ChecksCache {
	c := &ChecksCache{Entries: map[string]checksCacheEntry{}}
	if gitDir == "" {
		return c
	}
	data, err := os.ReadFile(ChecksCachePath(gitDir))
	if err != nil {
		return c
	}
	var loaded ChecksCache
	if err := json.Unmarshal(data, &loaded); err != nil {
		return c
	}
	if loaded.Entries == nil {
		loaded.Entries = map[string]checksCacheEntry{}
	}
	return &loaded
}

// Get returns the cached status for a PR number when the entry is
// still within the TTL.
func (c *ChecksCache) Get(number int) (PRStatus, bool) {
	if c == nil {
		return PRStatus{}, false
	}
	e, ok := c.Entries[fmt.Sprintf("%d", number)]
	if !ok {
		return PRStatus{}, false
	}
	if time.Since(e.FetchedAt) > checksCacheTTL {
		return PRStatus{}, false
	}
	return e.Status, true
}

// Put stores a status under the PR number with the current timestamp.
func (c *ChecksCache) Put(number int, st PRStatus) {
	if c == nil || c.Entries == nil {
		return
	}
	c.Entries[fmt.Sprintf("%d", number)] = checksCacheEntry{Status: st, FetchedAt: time.Now()}
}

// Save serialises the cache to its canonical path, creating the
// stac-man subdirectory if needed. Errors are returned but callers
// typically swallow them — a failed cache write should not break
// rendering.
func (c *ChecksCache) Save(gitDir string) error {
	if gitDir == "" || c == nil {
		return nil
	}
	p := ChecksCachePath(gitDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// InvalidateChecksCache removes the cache file on disk. Called after
// `sm submit` and `sm sync` because both can flip CI state and
// mergeability — keeping the stale entry would mean the next
// `sm log` shows the pre-mutation status.
func InvalidateChecksCache(gitDir string) error {
	if gitDir == "" {
		return nil
	}
	err := os.Remove(ChecksCachePath(gitDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
