package gh

import (
	"context"
	"encoding/json"
	"fmt"
)

// PRRef is a slim PR identifier used to walk a stack's base-ref chain.
type PRRef struct {
	Number  int
	Base    string // base branch on GitHub (e.g. main, or a feature branch)
	Head    string // head branch on GitHub
	URL     string
}

// PRView fetches just the fields needed to walk a stack.
func (c *Client) prRef(ctx context.Context, number int) (PRRef, error) {
	out, _, err := c.r.Run(ctx, "pr", "view", fmt.Sprintf("%d", number),
		"--json", "number,baseRefName,headRefName,url")
	if err != nil {
		return PRRef{}, err
	}
	var raw struct {
		Number      int    `json:"number"`
		BaseRefName string `json:"baseRefName"`
		HeadRefName string `json:"headRefName"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return PRRef{}, fmt.Errorf("parsing gh pr view: %w", err)
	}
	return PRRef{
		Number: raw.Number,
		Base:   raw.BaseRefName,
		Head:   raw.HeadRefName,
		URL:    raw.URL,
	}, nil
}

// StackForPR walks the base_ref chain starting at `topPR` until the
// base ref is the repo's default branch (or an unfound PR), and returns
// the chain in bottom-first order (so callers can checkout the
// bottom branch first, set its parent to trunk, then walk upward).
//
// trunkBranch is the repo's trunk; the walk stops when it sees this
// as a base. If the head PR's chain refers to PRs we can't fetch, the
// walk stops at the last known PR.
func (c *Client) StackForPR(ctx context.Context, topPR int, trunkBranch string) ([]PRRef, error) {
	if topPR <= 0 {
		return nil, fmt.Errorf("invalid PR number %d", topPR)
	}

	chain := []PRRef{}
	visited := map[int]bool{}
	cur, err := c.prRef(ctx, topPR)
	if err != nil {
		return nil, err
	}
	chain = append(chain, cur)
	visited[cur.Number] = true

	// Walk downward by querying the PR whose head is our base.
	for cur.Base != "" && cur.Base != trunkBranch {
		next, ok, err := c.prByHead(ctx, cur.Base)
		if err != nil {
			return nil, err
		}
		if !ok {
			// Base branch doesn't have an open PR — chain ends here.
			break
		}
		if visited[next.Number] {
			break // cycle guard
		}
		visited[next.Number] = true
		chain = append(chain, next)
		cur = next
	}

	// Reverse to bottom-first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// prByHead returns the most recent PR whose head matches branch. ok=
// false when there's no such PR.
func (c *Client) prByHead(ctx context.Context, branch string) (PRRef, bool, error) {
	out, _, err := c.r.Run(ctx, "pr", "list",
		"--head", branch,
		"--state", "all",
		"--limit", "1",
		"--json", "number,baseRefName,headRefName,url",
	)
	if err != nil {
		return PRRef{}, false, err
	}
	var raw []struct {
		Number      int    `json:"number"`
		BaseRefName string `json:"baseRefName"`
		HeadRefName string `json:"headRefName"`
		URL         string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return PRRef{}, false, fmt.Errorf("parsing gh pr list: %w", err)
	}
	if len(raw) == 0 {
		return PRRef{}, false, nil
	}
	r := raw[0]
	return PRRef{
		Number: r.Number,
		Base:   r.BaseRefName,
		Head:   r.HeadRefName,
		URL:    r.URL,
	}, true, nil
}

// PRCheckout runs `gh pr checkout <n>`. Creates the local branch if it
// doesn't already exist, and switches HEAD to it.
func (c *Client) PRCheckout(ctx context.Context, number int) error {
	_, _, err := c.r.Run(ctx, "pr", "checkout", fmt.Sprintf("%d", number))
	return err
}
