package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/restack"
	"github.com/philipptpunkt/stac-man/internal/stack"
)

// SyncReport summarizes what happened during a sync. The caller (cmd
// layer) decides how to render it.
type SyncReport struct {
	Trunk           string
	MergedBranches  []string // deleted because their commits are in trunk
	RestackedBranch string   // root of any restack performed (empty if none)
}

// Sync fetches trunk, fast-forwards the local trunk, deletes branches
// whose commits are fully merged, re-parents their children, and
// restacks every survivor.
func (s *Service) Sync(ctx context.Context) (SyncReport, error) {
	r := SyncReport{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	r.Trunk = trunk

	if clean, err := s.G.IsClean(ctx); err != nil {
		return r, err
	} else if !clean {
		return r, errors.New("working tree is dirty; commit or stash first")
	}

	originalBranch, _ := s.G.CurrentBranch(ctx)

	if err := s.G.FetchAll(ctx); err != nil {
		return r, fmt.Errorf("fetching: %w", err)
	}

	// Fast-forward the local trunk via a temporary checkout. We need
	// to be on the trunk for `git pull --ff-only` to update it.
	if err := s.G.Checkout(ctx, trunk); err != nil {
		return r, fmt.Errorf("checking out %s: %w", trunk, err)
	}
	if err := s.G.Pull(ctx, trunk); err != nil {
		return r, fmt.Errorf("pulling %s: %w", trunk, err)
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}

	// Identify merged branches in topo order from leaves up so we can
	// delete safely. A branch is "merged" when it has zero commits
	// not already in trunk.
	merged, err := s.detectMergedBranches(ctx, g, trunk)
	if err != nil {
		return r, err
	}
	r.MergedBranches = merged

	for _, b := range merged {
		if err := s.untrackAndDelete(ctx, g, b, trunk); err != nil {
			return r, fmt.Errorf("untracking merged branch %s: %w", b, err)
		}
	}

	// Reload the graph after deletions, then restack every surviving
	// root to pick up the new trunk tip.
	g, err = stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}
	roots := g.Roots()
	for _, root := range roots {
		// Bump every root's ParentSHA to the new trunk tip BEFORE
		// running the restack engine, so the engine knows the parent
		// has moved.
		// (The engine itself reads stored ParentSHA vs. live tip.)
		// No-op here — left as a comment for clarity.
		_ = root
	}
	for _, root := range roots {
		if err := restack.New(s.G, s.Store).Restack(ctx, root.Name); err != nil {
			r.RestackedBranch = root.Name
			return r, err
		}
	}

	// Best-effort: hop back to the original branch if it still exists.
	if originalBranch != "" && !contains(merged, originalBranch) {
		if exists, _ := s.G.BranchExists(ctx, originalBranch); exists {
			_ = s.G.Checkout(ctx, originalBranch)
		}
	}
	return r, nil
}

// detectMergedBranches walks the tracked branches and returns those
// whose commits are entirely contained in trunk. Returned in deletion-
// safe order: leaves first.
func (s *Service) detectMergedBranches(ctx context.Context, g *stack.Graph, trunk string) ([]string, error) {
	// Topological order (parents before children). Reverse to get
	// leaves first.
	all, err := s.Store.ListTrackedBranches(ctx)
	if err != nil {
		return nil, err
	}
	merged := []string{}
	for _, name := range all {
		if name == trunk {
			continue
		}
		// Skip if the branch doesn't actually exist locally — broken
		// metadata is the doctor's problem, not sync's.
		if exists, _ := s.G.BranchExists(ctx, name); !exists {
			continue
		}
		ahead, err := s.G.CountCommitsAhead(ctx, name, trunk)
		if err != nil {
			// Don't fail the whole sync on one bad branch.
			continue
		}
		if ahead == 0 {
			merged = append(merged, name)
		}
	}
	// Order by depth (descendants first) so deletions don't orphan a
	// child before we re-parent it.
	depth := func(name string) int {
		d := 0
		cur := name
		seen := map[string]bool{}
		for {
			b, ok := g.Get(cur)
			if !ok || b.Parent == "" || b.Parent == trunk || seen[b.Parent] {
				return d
			}
			seen[b.Parent] = true
			cur = b.Parent
			d++
		}
	}
	// Sort merged by depth descending.
	for i := 1; i < len(merged); i++ {
		for j := i; j > 0 && depth(merged[j]) > depth(merged[j-1]); j-- {
			merged[j], merged[j-1] = merged[j-1], merged[j]
		}
	}
	return merged, nil
}

// untrackAndDelete removes a merged branch from the stack and from
// git. Children of the merged branch are re-parented onto its parent
// (or trunk).
func (s *Service) untrackAndDelete(ctx context.Context, g *stack.Graph, branch, trunk string) error {
	parent := trunk
	if b, ok := g.Get(branch); ok && b.Parent != "" {
		parent = b.Parent
	}
	parentSHA, err := s.G.RevParse(ctx, parent)
	if err != nil {
		return err
	}
	for _, child := range g.ChildrenOf(branch) {
		meta, _, err := s.Store.GetBranch(ctx, child.Name)
		if err != nil {
			return err
		}
		meta.Parent = parent
		meta.ParentSHA = parentSHA
		if err := s.Store.SetBranch(ctx, child.Name, meta); err != nil {
			return err
		}
	}
	if err := s.Store.UnsetBranch(ctx, branch); err != nil {
		return err
	}
	// Use force delete since the branch may not be merged into HEAD
	// (we just hopped to trunk so it should be, but force is safe
	// once we've confirmed CountCommitsAhead == 0).
	return s.G.DeleteBranch(ctx, branch, true)
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// Compile-time assertion the strings package is used.
var _ = strings.TrimSpace
