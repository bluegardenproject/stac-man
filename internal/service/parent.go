package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// Parent returns the parent of the named branch (or current branch
// when name is empty).
func (s *Service) Parent(ctx context.Context, branch string) (string, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return "", err
	}
	if branch == "" {
		var err error
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return "", err
		}
	}
	meta, ok, err := s.Store.GetBranch(ctx, branch)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("branch %q is not tracked", branch)
	}
	return meta.Parent, nil
}

// Children returns the children of the named branch (or current
// branch when name is empty), as a slice of branch names sorted
// alphabetically.
func (s *Service) Children(ctx context.Context, branch string) ([]string, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return nil, err
	}
	if branch == "" {
		var err error
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return nil, err
		}
	}
	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return nil, err
	}
	children := g.ChildrenOf(branch)
	out := make([]string, 0, len(children))
	for _, c := range children {
		out = append(out, c.Name)
	}
	return out, nil
}

// SetParent reassigns the parent of branch to newParent and triggers
// a restack so HEAD's history is rebuilt onto the new base.
func (s *Service) SetParent(ctx context.Context, branch, newParent string) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return err
	}
	if branch == "" {
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return err
		}
	}
	if branch == trunk {
		return errors.New("trunk has no parent to set")
	}
	if newParent == branch {
		return errors.New("a branch cannot be its own parent")
	}
	if newParent != trunk {
		if exists, err := s.G.BranchExists(ctx, newParent); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("new parent %q does not exist", newParent)
		}
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return err
	}
	if !g.IsTracked(branch) {
		return fmt.Errorf("branch %q is not tracked", branch)
	}
	// Refuse to create a cycle: newParent must not be `branch` or any
	// of its descendants.
	for _, d := range g.Descendants(branch) {
		if d.Name == newParent {
			return fmt.Errorf("cycle: %q is a descendant of %q", newParent, branch)
		}
	}

	meta, _, err := s.Store.GetBranch(ctx, branch)
	if err != nil {
		return err
	}

	s.recordHistory(ctx, "set-parent", fmt.Sprintf("%s onto %s", branch, newParent), branchAndDescendants(ctx, s, branch))

	// Update only the Parent name; intentionally leave ParentSHA at
	// its current value so the restack engine sees a mismatch
	// (recorded SHA vs. new parent's actual tip) and triggers a real
	// rebase. The engine writes the post-rebase tip back to ParentSHA
	// itself once the rebase succeeds. Pre-updating ParentSHA here
	// would defeat the engine's needsRebase check (recorded == new
	// tip → no-op) and silently leave history pointing at the old
	// parent's commits — see B5 in TESTRUN.md.
	meta.Parent = newParent
	if err := s.Store.SetBranch(ctx, branch, meta); err != nil {
		return err
	}

	// Restack to actually move history onto the new parent.
	return restack.New(s.G, s.Store).Restack(ctx, branch)
}

// Move reparents branch (default: current) onto newParent and
// restacks the entire subtree onto its new base. Descendants ride
// along through the restack engine's cascade — we only flip branch's
// parent metadata; children keep their existing parents and are
// rebased automatically because their parent's tip moves.
func (s *Service) Move(ctx context.Context, branch, newParent string) error {
	if newParent == "" {
		return errors.New("--onto is required")
	}
	if branch == "" {
		var err error
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return err
		}
	}
	return s.SetParent(ctx, branch, newParent)
}

// Fold squashes the current branch into its parent: every commit
// unique to `current` is replayed as one commit on the parent, the
// parent is checked out, the original branch is deleted, and any
// children are re-parented onto the (now-folded) parent.
func (s *Service) Fold(ctx context.Context, message string) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return err
	}
	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return err
	}
	if current == trunk {
		return errors.New("cannot fold trunk")
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return err
	}
	cur, ok := g.Get(current)
	if !ok {
		return fmt.Errorf("branch %q is not tracked", current)
	}
	parent := cur.Parent
	if parent == "" {
		return fmt.Errorf("branch %q has no recorded parent", current)
	}

	if clean, err := s.G.IsClean(ctx); err != nil {
		return err
	} else if !clean {
		return errors.New("working tree is dirty; commit or stash first")
	}

	// Capture commit list and tip BEFORE we mutate anything so we can
	// fail safely.
	currentTip, err := s.G.RevParse(ctx, current)
	if err != nil {
		return err
	}
	parentTip, err := s.G.RevParse(ctx, parent)
	if err != nil {
		return err
	}
	if currentTip == parentTip {
		return fmt.Errorf("nothing to fold: %q has no commits beyond %q", current, parent)
	}

	if message == "" {
		message = fmt.Sprintf("fold %s into %s", current, parent)
	}

	// Snapshot every branch the fold touches: current, parent, and
	// every child that will be reparented.
	touched := []string{current, parent}
	for _, child := range g.ChildrenOf(current) {
		touched = append(touched, child.Name)
	}
	s.recordHistory(ctx, "fold", current+" into "+parent, touched)

	// Switch to the parent and merge --squash from current. This
	// stages the cumulative diff without creating a merge commit.
	if err := s.G.Checkout(ctx, parent); err != nil {
		return fmt.Errorf("checking out parent: %w", err)
	}
	if err := s.G.MergeSquash(ctx, current); err != nil {
		// Best-effort: try to leave the user back on `current` if the
		// merge --squash bailed out before staging anything.
		_ = s.G.Checkout(ctx, current)
		return fmt.Errorf("merge --squash failed: %w", err)
	}
	if err := s.G.Commit(ctx, message, false, false); err != nil {
		return fmt.Errorf("commit on parent: %w", err)
	}

	// Re-parent every child of `current` to `parent` and bump SHAs.
	newParentTip, err := s.G.RevParse(ctx, parent)
	if err != nil {
		return err
	}
	for _, child := range g.ChildrenOf(current) {
		meta, _, err := s.Store.GetBranch(ctx, child.Name)
		if err != nil {
			return err
		}
		meta.Parent = parent
		meta.ParentSHA = newParentTip
		if err := s.Store.SetBranch(ctx, child.Name, meta); err != nil {
			return err
		}
	}

	// Drop tracking and delete the local branch.
	if err := s.Store.UnsetBranch(ctx, current); err != nil {
		return fmt.Errorf("untracking %s: %w", current, err)
	}
	if err := s.G.DeleteBranch(ctx, current, true); err != nil {
		return fmt.Errorf("deleting %s: %w", current, err)
	}

	// Restack the (formerly) child branches so their history is rebuilt
	// onto the post-fold parent.
	for _, child := range g.ChildrenOf(current) {
		if err := restack.New(s.G, s.Store).Restack(ctx, child.Name); err != nil {
			return err
		}
	}

	return nil
}

// Adapter so the file references store.RepoMeta to keep imports tidy.
var _ = store.RepoMeta{}
