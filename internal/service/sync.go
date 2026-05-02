package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/restack"
	"github.com/philipptpunkt/stac-man/internal/stack"
	"github.com/philipptpunkt/stac-man/internal/ui/progress"
)

// SyncOptions configures Sync.
type SyncOptions struct {
	// Progress receives one step per long-running phase (fetch,
	// pull, gh PR-state lookup, per-PR retargets, per-root restack).
	// Nil collapses to a discard reporter so non-cmd callers (e.g.
	// `sm land`) keep their current quiet behaviour.
	Progress progress.Reporter
}

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

// mergedByPRState returns the names of tracked branches in g whose
// PR is reported as MERGED on GitHub.
//
// This is the seam that catches squash-merge and merge-commit
// landings that the rev-list-based detector cannot see: in both
// strategies GitHub creates a NEW commit on trunk, so the local
// branch's tip is no longer reachable from trunk and
// `git rev-list --count trunk..branch` stays >0 even though the
// branch is, semantically, fully merged.
//
// Pure: takes a pre-fetched PR map keyed by head branch (typically
// built via gh.PRsForBranches). The `exclude` set is used to skip
// branches the caller has already classified as merged via another
// signal — keeps the union deduped without a second pass.
func mergedByPRState(g *stack.Graph, prsByBranch map[string]gh.PR, exclude map[string]bool) []string {
	var out []string
	for _, b := range g.Branches() {
		if b.PR == 0 {
			// No local PR record → nothing to look up. (We could
			// still query gh by branch name here, but that turns
			// every sync into a per-branch round-trip even on
			// stacks that haven't called `sm submit` yet.)
			continue
		}
		if exclude[b.Name] {
			continue
		}
		pr, ok := prsByBranch[b.Name]
		if !ok {
			continue
		}
		if pr.State != gh.PRStateMerged {
			continue
		}
		out = append(out, b.Name)
	}
	return out
}

// sortMergedByDepth returns names ordered descendants-first relative
// to trunk, so callers can delete leaves before their parents.
// Extracted so both rev-list-based and PR-state-based detection
// feed the same ordering pipeline.
func sortMergedByDepth(g *stack.Graph, names []string, trunk string) []string {
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
	out := append([]string(nil), names...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && depth(out[j]) > depth(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// Sync fetches trunk, fast-forwards the local trunk, deletes branches
// whose commits are fully merged, re-parents their children, and
// restacks every survivor.
func (s *Service) Sync(ctx context.Context, opts SyncOptions) (SyncReport, error) {
	r := SyncReport{}
	prog := opts.Progress
	if prog == nil {
		prog = progress.Discard()
	}
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

	prog.Start("fetching from origin")
	if err := s.G.FetchAll(ctx); err != nil {
		prog.Fail("fetching from origin")
		return r, fmt.Errorf("fetching: %w", err)
	}
	prog.Done("fetched from origin")

	// Fast-forward the local trunk via a temporary checkout. We need
	// to be on the trunk for `git pull --ff-only` to update it.
	if err := s.G.Checkout(ctx, trunk); err != nil {
		return r, fmt.Errorf("checking out %s: %w", trunk, err)
	}
	prog.Start(fmt.Sprintf("pulling %s", trunk))
	if err := s.G.Pull(ctx, trunk); err != nil {
		prog.Fail(fmt.Sprintf("pulling %s", trunk))
		return r, fmt.Errorf("pulling %s: %w", trunk, err)
	}
	prog.Done(fmt.Sprintf("pulled %s", trunk))

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}

	// Identify merged branches in two passes. Pass 1 (rev-list)
	// catches fast-forward and rebase-merge landings where the
	// local branch's tip is reachable from trunk. Pass 2 (PR state)
	// catches squash- and merge-commit landings where the local
	// branch's tip differs from the (squashed/merged) commit GitHub
	// wrote to trunk — without this pass, `sm sync` declares the
	// stack "up to date" while leaving merged branches around.
	mergedByHistory, err := s.detectMergedBranches(ctx, g, trunk)
	if err != nil {
		return r, err
	}
	excluded := make(map[string]bool, len(mergedByHistory))
	for _, n := range mergedByHistory {
		excluded[n] = true
	}

	// Construct the gh client once, lazily — only when there is at
	// least one tracked branch with a PR record. Repos that haven't
	// called `sm submit` yet still don't need gh auth to run sync.
	var ghClient *gh.Client
	var mergedByPR []string
	if hasAnyTrackedPR(g) {
		ghClient = gh.New("")
		// Best-effort: a single failed gh round-trip shouldn't take
		// down the whole sync. We just lose squash-merge detection
		// for this run and fall back to history-only.
		prog.Start("checking PR merge state on GitHub")
		prsByBranch, ghErr := s.fetchTrackedPRs(ctx, g, ghClient, excluded)
		if ghErr == nil {
			prog.Done("checked PR merge state on GitHub")
			mergedByPR = mergedByPRState(g, prsByBranch, excluded)
		} else {
			prog.Fail("checking PR merge state on GitHub")
		}
	}

	merged := append([]string{}, mergedByHistory...)
	merged = append(merged, mergedByPR...)
	merged = sortMergedByDepth(g, merged, trunk)
	r.MergedBranches = merged

	// Phase 1: rewrite local metadata (delete merged branches and
	// cascade-reparent surviving descendants). Network-free so a
	// failure here can't leave GitHub partially updated. The helper
	// reloads the graph between iterations so cascade reparenting
	// follows the chain through every layer.
	retargetEntries, err := s.processMergedBranches(ctx, merged, trunk)
	if err != nil {
		return r, err
	}

	// Phase 2: tell GitHub about the new parent edges. EditPR
	// failures are recorded per-entry but never abort the run —
	// flaky network is not a reason to undo the local cleanup
	// we just finished. Each retarget gets its own progress step
	// so the spinner in interactive shells (or the plain ✓ / ✗
	// line in pipes) shows the user which PR is being touched.
	if len(retargetEntries) > 0 && ghClient == nil {
		// We arrive here only when the merged set has child PRs
		// but no other tracked branch had a PR record before —
		// rare but possible if metadata was hand-edited. Construct
		// gh on demand so retargeting still works.
		ghClient = gh.New("")
	}
	for _, e := range retargetEntries {
		rp := RetargetedPR{Branch: e.Branch, PR: e.PR, NewBase: e.NewBase}
		label := fmt.Sprintf("retargeting PR #%d → %s", e.PR, e.NewBase)
		prog.Start(label)
		if err := ghClient.EditPR(ctx, e.PR, gh.EditPROptions{Base: e.NewBase}); err != nil {
			rp.Err = err.Error()
			prog.Fail(label)
		} else {
			prog.Done(fmt.Sprintf("retargeted PR #%d → %s", e.PR, e.NewBase))
		}
		r.RetargetedPRs = append(r.RetargetedPRs, rp)
	}

	// Reload the graph after deletions, then restack every surviving
	// root to pick up the new trunk tip.
	g, err = stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}
	// The restack engine reads stored ParentSHA vs. the live parent
	// tip and rebases on mismatch. Walking from each root cascades
	// the rewrite down the chain via its `willMove` set, so
	// descendants pick up trunk's new tip without us touching their
	// metadata here.
	for _, root := range g.Roots() {
		// Restack steps run last so a paused-conflict ends with the
		// matching ✗ line — the user reads back from the bottom of
		// scrollback to see which root the conflict landed on.
		prog.Start(fmt.Sprintf("restacking %s", root.Name))
		if err := restack.New(s.G, s.Store).Restack(ctx, root.Name); err != nil {
			prog.Fail(fmt.Sprintf("restacking %s", root.Name))
			r.RestackedBranch = root.Name
			return r, err
		}
		prog.Done(fmt.Sprintf("restacked %s", root.Name))
	}

	// Best-effort: hop back to the original branch if it still exists.
	if originalBranch != "" && !contains(merged, originalBranch) {
		if exists, _ := s.G.BranchExists(ctx, originalBranch); exists {
			_ = s.G.Checkout(ctx, originalBranch)
		}
	}

	// Sync's restack cascade rewrites every descendant after a
	// merge upstream — both CI state and mergeability go stale on
	// GitHub the moment the next `sm submit` pushes, but the cache
	// from before sync would still claim the old state. Drop it so
	// `sm log` re-fetches on next render.
	if gitDir, err := s.G.GitDir(ctx); err == nil {
		_ = gh.InvalidateChecksCache(gitDir)
	}

	return r, nil
}

// detectMergedBranches walks the tracked branches and returns those
// whose commits are entirely contained in trunk. Catches FF and
// rebase-merge landings where the merged commit's SHA on trunk
// matches the local branch's tip; misses squash- and merge-commit
// landings (see mergedByPRState for that). Returned in
// deletion-safe order: leaves first.
func (s *Service) detectMergedBranches(ctx context.Context, g *stack.Graph, trunk string) ([]string, error) {
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
	return sortMergedByDepth(g, merged, trunk), nil
}

// hasAnyTrackedPR reports whether at least one branch in g has a
// stored PR number. Used to gate gh client construction so a fresh
// repo with no submissions can still run `sm sync` without needing
// gh auth.
func hasAnyTrackedPR(g *stack.Graph) bool {
	for _, b := range g.Branches() {
		if b.PR != 0 {
			return true
		}
	}
	return false
}

// fetchTrackedPRs returns a map of branch → PR for every tracked
// branch in g that (a) has a stored PR number and (b) is not in
// `skip` (branches we've already classified as merged via history,
// for which a gh round-trip is wasted work).
//
// We skip branches without a local PR record because there's
// nothing to look up by branch name in O(1); the alternative is
// one `gh pr list --head` call per branch which is too expensive
// to run on every sync.
func (s *Service) fetchTrackedPRs(ctx context.Context, g *stack.Graph, client *gh.Client, skip map[string]bool) (map[string]gh.PR, error) {
	var heads []string
	for _, b := range g.Branches() {
		if b.PR == 0 {
			continue
		}
		if skip[b.Name] {
			continue
		}
		heads = append(heads, b.Name)
	}
	if len(heads) == 0 {
		return map[string]gh.PR{}, nil
	}
	return client.PRsForBranches(ctx, heads)
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
	retargets := retargetPlan(g, branch, parent)
	for _, child := range g.ChildrenOf(branch) {
		meta, ok, err := s.Store.GetBranch(ctx, child.Name)
		if err != nil {
			return nil, err
		}
		if !ok {
			// The "child" is a ghost: it appears in the in-memory
			// graph snapshot the caller passed us, but its store
			// metadata has already been removed — typically by an
			// earlier untrackAndDelete call in the same Sync run
			// where this branch was a deeper merged ancestor. If we
			// fell through and called SetBranch we'd resurrect the
			// ghost with only Parent populated (no ParentSHA, no
			// PR), and the next Sync would see a tracked branch
			// with no local ref and crash on the rev-parse during
			// the restack walk. Skipping is safe: we already deleted
			// this branch's metadata, the caller (processMergedBranches)
			// reloads the graph between iterations so the next
			// pass sees the updated tree, and the actual surviving
			// descendant gets reparented through that fresh view.
			continue
		}
		// Update only the Parent name; intentionally leave ParentSHA
		// at its current value (the merged branch's tip from the
		// child's POV). The restack engine reads stored ParentSHA
		// vs. live tip — for squash- and merge-commit landings the
		// new parent's tip differs from the old recorded SHA, so
		// the engine's mismatch check fires and rebases the child
		// from the right base. Pre-updating ParentSHA here would
		// silence that signal and leave the child's history
		// pointing at commits that no longer exist on trunk —
		// the same anti-pattern as B5 in TESTRUN.md, just inside
		// sync's cleanup path. For FF/rebase-merge the old SHA
		// already coincides with the new parent's tip, so this
		// preservation is a no-op there.
		meta.Parent = parent
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

// processMergedBranches deletes each merged branch in order, reloading
// the stack graph between iterations so each iteration sees the
// post-deletion topology produced by the previous one. Returns the
// accumulated PR-retarget plan (one entry per child PR whose base
// must move to its new ancestor on GitHub).
//
// Why reload between iterations: cleanup proceeds deepest-first, so
// later iterations need to know about reparenting decisions made by
// earlier ones — otherwise a surviving descendant whose chain runs
// through several merged ancestors loses every step of the cascade
// after the first, and ends up pointing at a deleted parent. The
// per-iteration reload is cheap (the store is local git config) and
// keeps the cascade O(merged) instead of O(merged²) in the number
// of repeated SetBranch calls a single-pass approach would require
// to fix up survivors at every layer.
//
// gh PR retargets are NOT issued from here — the returned plan is
// applied separately by Sync so the metadata-rewrite phase stays
// network-free and a partially-failed gh round-trip leaves local
// state fully consistent.
func (s *Service) processMergedBranches(ctx context.Context, merged []string, trunk string) ([]retargetEntry, error) {
	var plan []retargetEntry
	for _, b := range merged {
		g, err := stack.Load(ctx, s.Store)
		if err != nil {
			return plan, err
		}
		retargets, err := s.untrackAndDelete(ctx, g, b, trunk)
		if err != nil {
			return plan, fmt.Errorf("untracking merged branch %s: %w", b, err)
		}
		plan = append(plan, retargets...)
	}
	return plan, nil
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
