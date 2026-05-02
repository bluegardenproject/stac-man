package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// TrackOptions configures Track.
type TrackOptions struct {
	// Branch to track. Empty means the current branch.
	Branch string
	// Parent is the explicit parent. Empty means infer the parent by
	// merge-base against trunk and any already-tracked branches.
	Parent string
}

// Track adopts a branch into the stack graph. Useful when a branch was
// created with plain `git checkout -b` before stac-man knew about it.
func (s *Service) Track(ctx context.Context, opts TrackOptions) (string, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return "", err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return "", err
	}

	branch := opts.Branch
	if branch == "" {
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return "", err
		}
	}
	if branch == trunk {
		return "", errors.New("refusing to track the trunk branch")
	}

	if exists, err := s.G.BranchExists(ctx, branch); err != nil {
		return "", err
	} else if !exists {
		return "", fmt.Errorf("branch %q does not exist", branch)
	}

	parent := opts.Parent
	if parent == "" {
		parent, err = s.inferParent(ctx, branch, trunk)
		if err != nil {
			return "", err
		}
	} else if parent != trunk {
		// User-supplied parent must itself exist.
		if exists, err := s.G.BranchExists(ctx, parent); err != nil {
			return "", err
		} else if !exists {
			return "", fmt.Errorf("parent branch %q does not exist", parent)
		}
	}

	parentSHA, err := s.G.RevParse(ctx, parent)
	if err != nil {
		return "", fmt.Errorf("resolving parent SHA: %w", err)
	}

	s.recordHistory(ctx, "track", branch, []string{branch})

	meta := store.BranchMeta{Parent: parent, ParentSHA: parentSHA}
	// Preserve an existing PR number so re-tracking doesn't lose it.
	if existing, ok, err := s.Store.GetBranch(ctx, branch); err == nil && ok {
		meta.PR = existing.PR
	}
	if err := s.Store.SetBranch(ctx, branch, meta); err != nil {
		return "", fmt.Errorf("recording stack metadata: %w", err)
	}
	return parent, nil
}

// inferParent picks the candidate branch with the deepest merge-base
// against `branch` (i.e. the most-recent common ancestor). Trunk is
// always a candidate so a fresh root branch resolves to the trunk.
func (s *Service) inferParent(ctx context.Context, branch, trunk string) (string, error) {
	tracked, err := s.Store.ListTrackedBranches(ctx)
	if err != nil {
		return "", err
	}

	candidates := make([]string, 0, len(tracked)+1)
	candidates = append(candidates, trunk)
	for _, t := range tracked {
		if t != branch {
			candidates = append(candidates, t)
		}
	}

	type scored struct {
		name string
		sha  string
	}
	var best *scored
	for _, c := range candidates {
		mb, err := s.G.MergeBase(ctx, branch, c)
		if err != nil {
			continue
		}
		// The candidate whose merge-base is closest to its own tip is
		// the best parent. Use IsAncestor(mb, candidateTip) to check
		// the merge-base reaches candidate's tip — when it does, the
		// candidate is fully merged into branch and is a valid parent.
		// Among such candidates we prefer the one with the latest
		// merge-base by commit topology (IsAncestor of others).
		if best == nil {
			best = &scored{name: c, sha: mb}
			continue
		}
		// Prefer a candidate whose merge-base is a descendant of the
		// current best merge-base. That means it's deeper in the
		// graph and therefore a closer parent.
		isCloser, err := s.G.IsAncestor(ctx, best.sha, mb)
		if err == nil && isCloser {
			best = &scored{name: c, sha: mb}
		}
	}
	if best == nil {
		return "", errors.New("could not infer parent; pass --parent <branch>")
	}
	return best.name, nil
}

// Untrack removes a branch from the stack graph. If the branch has
// tracked children, they are re-parented to the branch's own parent
// when reparent=true, otherwise Untrack returns an error.
func (s *Service) Untrack(ctx context.Context, branch string, reparent bool) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}

	if branch == "" {
		var err error
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return err
		}
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return err
	}
	if !g.IsTracked(branch) {
		return fmt.Errorf("branch %q is not tracked", branch)
	}

	children := g.ChildrenOf(branch)
	if len(children) > 0 && !reparent {
		return fmt.Errorf("branch %q has %d tracked children; pass --reparent to move them onto its parent", branch, len(children))
	}

	touched := []string{branch}
	for _, child := range children {
		touched = append(touched, child.Name)
	}
	s.recordHistory(ctx, "untrack", branch, touched)

	if reparent && len(children) > 0 {
		parent, _ := g.Get(branch)
		newParent := parent.Parent
		newParentSHA, err := s.G.RevParse(ctx, newParent)
		if err != nil {
			return fmt.Errorf("resolving SHA for new parent %s: %w", newParent, err)
		}
		for _, child := range children {
			meta, _, err := s.Store.GetBranch(ctx, child.Name)
			if err != nil {
				return err
			}
			meta.Parent = newParent
			meta.ParentSHA = newParentSHA
			if err := s.Store.SetBranch(ctx, child.Name, meta); err != nil {
				return err
			}
		}
	}

	return s.Store.UnsetBranch(ctx, branch)
}
