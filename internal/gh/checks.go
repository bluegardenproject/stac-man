package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
