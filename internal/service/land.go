package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/stack"
)

// LandOptions configures Land.
type LandOptions struct {
	// Method selects how the PR is merged on GitHub. Default: squash.
	Method gh.MergeMethod
	// Force skips the CI-green gate.
	Force bool
}

// LandReport is what Land returns to the cmd layer.
type LandReport struct {
	Branch string
	PR     int
	URL    string
	Method gh.MergeMethod
	Sync   SyncReport
	Synced bool
}

// Land merges the bottom-most tracked branch on the path from trunk
// to current, then runs Sync to clean up the merged branch and reparent
// its children onto trunk. Refuses unless that branch's PR exists, is
// open, and CI is green (use --force to skip the CI gate).
func (s *Service) Land(ctx context.Context, opts LandOptions) (LandReport, error) {
	r := LandReport{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	if err := gh.PreflightCheck(ctx); err != nil {
		return r, err
	}

	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return r, err
	}
	if current == trunk {
		return r, errors.New("refusing to land trunk")
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}
	if !g.IsTracked(current) {
		return r, fmt.Errorf("branch %q is not tracked", current)
	}

	bottom := bottomMost(g, trunk, current)
	if bottom == "" {
		return r, fmt.Errorf("no tracked branch on the path from %s to %s", trunk, current)
	}
	r.Branch = bottom

	method := opts.Method
	if method == "" {
		method = gh.MergeSquash
	}
	r.Method = method

	client := gh.New("")
	pr, ok, err := client.PRForBranch(ctx, bottom)
	if err != nil {
		return r, err
	}
	if !ok {
		return r, fmt.Errorf("branch %q has no PR; run `sm submit` first", bottom)
	}
	if pr.State != gh.PRStateOpen {
		return r, fmt.Errorf("PR #%d for %s is %s, not open", pr.Number, bottom, pr.State)
	}
	r.PR = pr.Number
	r.URL = pr.URL

	if !opts.Force {
		rollup, err := client.PRChecks(ctx, pr.Number)
		if err != nil {
			return r, fmt.Errorf("reading PR checks: %w", err)
		}
		switch rollup {
		case gh.ChecksFail:
			return r, fmt.Errorf("PR #%d has failing checks; pass --force to land anyway", pr.Number)
		case gh.ChecksPending:
			return r, fmt.Errorf("PR #%d still has pending checks; wait or pass --force", pr.Number)
		}
	}

	tracked, _ := s.Store.ListTrackedBranches(ctx)
	s.recordHistory(ctx, "land", fmt.Sprintf("#%d via %s", pr.Number, method), append([]string{trunk}, tracked...))

	if err := client.MergePR(ctx, pr.Number, method); err != nil {
		return r, fmt.Errorf("merging PR #%d: %w", pr.Number, err)
	}

	// Sync handles fetching trunk, deleting the merged branch locally,
	// reparenting its children, and restacking the survivors.
	sync, err := s.Sync(ctx)
	if err != nil {
		return r, fmt.Errorf("merged but post-merge sync failed: %w", err)
	}
	r.Sync = sync
	r.Synced = true
	return r, nil
}

// bottomMost returns the branch closest to trunk on the path from
// trunk -> current. Walks parents from current until just before trunk.
func bottomMost(g *stack.Graph, trunk, current string) string {
	cur := current
	seen := map[string]bool{}
	for {
		if seen[cur] {
			return ""
		}
		seen[cur] = true
		b, ok := g.Get(cur)
		if !ok {
			return ""
		}
		if b.Parent == "" || b.Parent == trunk {
			return cur
		}
		cur = b.Parent
	}
}
