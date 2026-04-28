package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/restack"
	"github.com/philipptpunkt/stac-man/internal/stack"
)

// SyncReport summarizes what happened during a sync. The caller (cmd
// layer) decides how to render it.
type SyncReport struct {
	Trunk           string
	MergedBranches  []string       // deleted because their commits are in trunk
	RetargetedPRs   []RetargetedPR // child PR bases updated on GitHub after a parent merged
	RestackedBranch string         // root of any restack performed (empty if none)
}

// RetargetedPR records one `gh pr edit --base` call attempted while
// cleaning up after a merged branch. Err is empty on success and
// otherwise carries the failure reason — sync deliberately keeps
// these errors non-fatal so a flaky network or missing gh auth
// doesn't undo the local re-parenting we already finished.
type RetargetedPR struct {
	Branch  string
	PR      int
	NewBase string
	Err     string
}

// retargetEntry is the unit-testable plan of "if local re-parenting
// succeeded, here is the matching `gh pr edit` call we would issue".
// Kept separate from RetargetedPR so the planner stays a pure function
// of the stack graph and never touches the network.
type retargetEntry struct {
	Branch  string
	PR      int
	NewBase string
}

// retargetPlan returns the child-PR retargets that follow from
// removing `merged` and re-parenting its direct children onto
// `newParent`. Children without an associated PR (`PR == 0`) are
// filtered out — the live runner can't retarget what doesn't exist
// on GitHub yet. We deliberately don't try to dedupe against an
// already-correct base here: `gh pr edit --base` is idempotent and
// a single extra round-trip per merged branch is cheaper than
// fetching every child's current base just to skip it.
//
// The function is pure — pass in the loaded graph and let the caller
// decide when to consult the network. This is the seam tests use to
// lock down the mapping between merged-branch cleanup and PR
// retargeting without spinning up a fake gh runner.
func retargetPlan(g *stack.Graph, merged, newParent string) []retargetEntry {
	var out []retargetEntry
	for _, child := range g.ChildrenOf(merged) {
		if child.PR == 0 {
			continue
		}
		out = append(out, retargetEntry{
			Branch:  child.Name,
			PR:      child.PR,
			NewBase: newParent,
		})
	}
	return out
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

	// Snapshot every tracked branch up-front so undo can revert merges,
	// reparenting, and restacks together.
	tracked, _ := s.Store.ListTrackedBranches(ctx)
	s.recordHistory(ctx, "sync", "", append([]string{trunk}, tracked...))

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

	// Construct the gh client once, lazily — only when there is at
	// least one merged branch with child PRs to retarget. Avoids
	// requiring `gh` auth on every sync just to get past this point.
	var ghClient *gh.Client
	for _, b := range merged {
		retargets, err := s.untrackAndDelete(ctx, g, b, trunk)
		if err != nil {
			return r, fmt.Errorf("untracking merged branch %s: %w", b, err)
		}
		if len(retargets) == 0 {
			continue
		}
		if ghClient == nil {
			ghClient = gh.New("")
		}
		for _, e := range retargets {
			rp := RetargetedPR{Branch: e.Branch, PR: e.PR, NewBase: e.NewBase}
			if err := ghClient.EditPR(ctx, e.PR, gh.EditPROptions{Base: e.NewBase}); err != nil {
				rp.Err = err.Error()
			}
			r.RetargetedPRs = append(r.RetargetedPRs, rp)
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
//
// Returns the list of PR base retargets the caller should issue
// against GitHub. We compute the plan from the stack graph BEFORE
// mutating it so callers don't need to re-load the graph just to
// figure out which children had PRs.
func (s *Service) untrackAndDelete(ctx context.Context, g *stack.Graph, branch, trunk string) ([]retargetEntry, error) {
	parent := trunk
	if b, ok := g.Get(branch); ok && b.Parent != "" {
		parent = b.Parent
	}
	parentSHA, err := s.G.RevParse(ctx, parent)
	if err != nil {
		return nil, err
	}
	retargets := retargetPlan(g, branch, parent)
	for _, child := range g.ChildrenOf(branch) {
		meta, _, err := s.Store.GetBranch(ctx, child.Name)
		if err != nil {
			return nil, err
		}
		meta.Parent = parent
		meta.ParentSHA = parentSHA
		if err := s.Store.SetBranch(ctx, child.Name, meta); err != nil {
			return nil, err
		}
	}
	if err := s.Store.UnsetBranch(ctx, branch); err != nil {
		return nil, err
	}
	// Use force delete since the branch may not be merged into HEAD
	// (we just hopped to trunk so it should be, but force is safe
	// once we've confirmed CountCommitsAhead == 0).
	if err := s.G.DeleteBranch(ctx, branch, true); err != nil {
		return nil, err
	}
	return retargets, nil
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
