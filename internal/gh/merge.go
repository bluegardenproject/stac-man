package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MergeMethod is which strategy `gh pr merge` uses.
type MergeMethod string

const (
	MergeSquash  MergeMethod = "squash"
	MergeCommit  MergeMethod = "merge"
	MergeRebase  MergeMethod = "rebase"
)

// MergePR merges a PR via `gh pr merge`. We pass --delete-branch=false
// because stac-man's Sync handles local-branch deletion in its
// merged-detection pass; letting gh delete the local branch races
// with our metadata update.
func (c *Client) MergePR(ctx context.Context, number int, method MergeMethod) error {
	if number <= 0 {
		return fmt.Errorf("invalid PR number %d", number)
	}
	flag := "--squash"
	switch method {
	case MergeCommit:
		flag = "--merge"
	case MergeRebase:
		flag = "--rebase"
	case MergeSquash, "":
		flag = "--squash"
	default:
		return fmt.Errorf("unknown merge method %q", method)
	}
	_, _, err := c.r.Run(ctx, "pr", "merge", fmt.Sprintf("%d", number), flag, "--delete-branch=false")
	return err
}

// CheckRollup is the aggregate status of CI checks for a PR.
type CheckRollup string

const (
	ChecksPass    CheckRollup = "PASS"    // every required check is green
	ChecksFail    CheckRollup = "FAIL"    // at least one required check is red
	ChecksPending CheckRollup = "PENDING" // some checks haven't completed yet
	ChecksNone    CheckRollup = "NONE"    // PR has no checks configured
)

// PRChecks queries `gh pr checks <n> --json state` and folds the per-
// check states into a single rollup. We classify any FAILURE / TIMED_OUT
// / CANCELLED / ERROR as a failure; SUCCESS or SKIPPED as pass; anything
// else as pending. NEUTRAL counts as pass.
func (c *Client) PRChecks(ctx context.Context, number int) (CheckRollup, error) {
	out, _, err := c.r.Run(ctx, "pr", "checks", fmt.Sprintf("%d", number), "--json", "state,conclusion")
	if err != nil {
		// `gh pr checks` exits 1 when checks are failing — but also
		// when there are no checks. Differentiate via parsing.
		// gh prints JSON to stdout even on failure exits.
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return ChecksNone, nil
	}
	var raw []struct {
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	}
	if jerr := json.Unmarshal([]byte(out), &raw); jerr != nil {
		// gh older versions print a tabular format on this command
		// — fall back to a non-strict heuristic: treat parse errors
		// as ChecksPending.
		return ChecksPending, nil
	}
	if len(raw) == 0 {
		return ChecksNone, nil
	}
	hasPending := false
	for _, ch := range raw {
		state := strings.ToUpper(ch.State)
		concl := strings.ToUpper(ch.Conclusion)
		switch concl {
		case "FAILURE", "TIMED_OUT", "CANCELLED", "ERROR", "ACTION_REQUIRED":
			return ChecksFail, nil
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			continue
		}
		switch state {
		case "FAILURE", "ERROR":
			return ChecksFail, nil
		case "PENDING", "QUEUED", "IN_PROGRESS":
			hasPending = true
		}
	}
	if hasPending {
		return ChecksPending, nil
	}
	return ChecksPass, nil
}
