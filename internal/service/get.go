package service

import (
	"context"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// GetReport summarizes the result of fetching a remote stack locally.
type GetReport struct {
	Trunk    string
	Branches []string // bottom-first order
	Top      string
}

// Get fetches every PR in the stack rooted at topPR and reproduces
// the parent edges in local stac-man metadata. After this returns
// successfully, HEAD is on the topmost branch of the stack and the
// graph is set up so `sm log` shows the same shape the PR author had.
func (s *Service) Get(ctx context.Context, topPR int) (GetReport, error) {
	r := GetReport{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	r.Trunk = trunk
	if err := gh.PreflightCheck(ctx); err != nil {
		return r, err
	}

	client := gh.New("")
	chain, err := client.StackForPR(ctx, topPR, trunk)
	if err != nil {
		return r, fmt.Errorf("walking PR chain: %w", err)
	}
	if len(chain) == 0 {
		return r, fmt.Errorf("PR #%d not found", topPR)
	}

	heads := make([]string, 0, len(chain))
	for _, pr := range chain {
		heads = append(heads, pr.Head)
	}
	s.recordHistory(ctx, "get", fmt.Sprintf("PR #%d", topPR), heads)

	// Fetch first so PR refs are reachable. Best-effort: don't fail
	// the whole command if fetch errors (e.g. offline) — gh will
	// surface its own error on checkout.
	_ = s.G.FetchAll(ctx)

	// Walk bottom-first: checkout each PR's branch and record its
	// parent metadata pointing at the previous (deeper) branch's
	// tip. The deepest branch's parent is trunk.
	previous := trunk
	for _, pr := range chain {
		if err := client.PRCheckout(ctx, pr.Number); err != nil {
			return r, fmt.Errorf("checking out PR #%d: %w", pr.Number, err)
		}
		parentSHA, err := s.G.RevParse(ctx, previous)
		if err != nil {
			return r, fmt.Errorf("resolving %s: %w", previous, err)
		}
		meta := store.BranchMeta{Parent: previous, ParentSHA: parentSHA, PR: pr.Number}
		// Preserve any existing PR number etc. by merging.
		if existing, ok, err := s.Store.GetBranch(ctx, pr.Head); err == nil && ok {
			if meta.PR == 0 {
				meta.PR = existing.PR
			}
		}
		if err := s.Store.SetBranch(ctx, pr.Head, meta); err != nil {
			return r, fmt.Errorf("recording parent for %s: %w", pr.Head, err)
		}
		r.Branches = append(r.Branches, pr.Head)
		previous = pr.Head
	}
	r.Top = previous
	return r, nil
}
