package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// branchAndDescendants returns name plus its tracked descendants.
// Used by recordHistory call sites that affect a whole subtree.
// Errors are swallowed — history is best-effort.
func branchAndDescendants(ctx context.Context, s *Service, name string) []string {
	out := []string{name}
	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return out
	}
	for _, d := range g.Descendants(name) {
		out = append(out, d.Name)
	}
	return out
}

// Restack rebases the current (or named) branch and every tracked
// descendant onto its parent's current tip.
func (s *Service) Restack(ctx context.Context, branch string) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	if _, err := s.EnsureTrunk(ctx); err != nil {
		return err
	}
	if branch == "" {
		var err error
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return err
		}
	}
	s.recordHistory(ctx, "restack", branch, branchAndDescendants(ctx, s, branch))
	return restack.New(s.G, s.Store).Restack(ctx, branch)
}

// RestackContinue resumes a paused restack after the user has
// resolved conflicts and staged the resolution.
func (s *Service) RestackContinue(ctx context.Context) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	return restack.New(s.G, s.Store).Continue(ctx)
}

// RestackAbort discards an in-progress restack and restores the
// branch HEAD was on when it started.
func (s *Service) RestackAbort(ctx context.Context) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	return restack.New(s.G, s.Store).Abort(ctx)
}

// ModifyOptions configures Modify.
type ModifyOptions struct {
	// Commit creates a new commit from staged changes.
	Commit bool
	// Amend replaces the current branch tip with a new commit
	// containing the staged (or staged + unstaged when StageAll is
	// true) changes.
	Amend bool
	// StageAll stages tracked-but-modified files before committing,
	// matching `git commit -a` semantics. Untracked files are NOT
	// included unless IncludeUntracked is also true.
	StageAll bool
	// IncludeUntracked, in combination with StageAll, also stages
	// new (untracked) files. Off by default to avoid silently
	// absorbing unrelated work-in-progress.
	IncludeUntracked bool
	// Message is the commit message. Required when Commit=true and
	// Amend=false; optional otherwise.
	Message string
}

// stageWorkingTree stages working-tree changes for the upcoming
// commit. By default it mirrors `git commit -a`: tracked-but-modified
// files only. When includeUntracked is true it falls back to
// `git add -A` so genuinely-new files are picked up too.
func stageWorkingTree(ctx context.Context, g gitStager, includeUntracked bool) error {
	var err error
	if includeUntracked {
		err = g.AddAll(ctx)
	} else {
		err = g.AddUpdate(ctx)
	}
	if err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}
	return nil
}

// gitStager is the subset of git.Client used by stageWorkingTree.
// Pulling it out as a small interface keeps the helper trivially
// fakeable in tests without dragging in the full Client.
type gitStager interface {
	AddAll(ctx context.Context) error
	AddUpdate(ctx context.Context) error
}

// Modify amends the current commit (or creates a new one) and
// triggers a restack of all descendants so they pick up the rewritten
// history.
func (s *Service) Modify(ctx context.Context, opts ModifyOptions) error {
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
		return fmt.Errorf("refusing to modify trunk %q", trunk)
	}

	if opts.Amend && opts.Commit {
		return errors.New("--amend and --commit are mutually exclusive")
	}
	if opts.Commit && opts.Message == "" {
		return errors.New("--commit requires -m <message>")
	}

	// Guard: amending when the branch has no commits of its own would
	// rewrite a commit that belongs to the parent (or trunk) — silently
	// pulling its changes into this branch under a new SHA. Detect this
	// up front and tell the user to use -c.
	if opts.Amend {
		base := trunk
		if meta, ok, err := s.Store.GetBranch(ctx, current); err != nil {
			return err
		} else if ok {
			base = meta.Parent
		}
		ahead, err := s.G.CountCommitsAhead(ctx, current, base)
		if err != nil {
			return fmt.Errorf("checking commits ahead of %s: %w", base, err)
		}
		if ahead == 0 {
			return fmt.Errorf(
				"cannot amend: branch %q has no commits of its own (tip matches %s). Use `sm modify -c -m \"...\"` to add the first commit.",
				current, base,
			)
		}
	}

	s.recordHistory(ctx, "modify", current, branchAndDescendants(ctx, s, current))

	if opts.StageAll {
		if err := stageWorkingTree(ctx, s.G, opts.IncludeUntracked); err != nil {
			return err
		}
	}

	switch {
	case opts.Commit:
		if err := s.G.Commit(ctx, opts.Message, false, false); err != nil {
			return fmt.Errorf("committing: %w", err)
		}
	case opts.Amend:
		if err := s.G.Commit(ctx, opts.Message, true, false); err != nil {
			return fmt.Errorf("amending: %w", err)
		}
	default:
		return errors.New("nothing to do: pass --commit or --amend (and -m for messages)")
	}

	// After history changes on `current`, every descendant needs to
	// be rebased onto the new tip.
	meta, ok, err := s.Store.GetBranch(ctx, current)
	if err != nil {
		return err
	}
	if !ok {
		// User modified an untracked branch — nothing to restack.
		return nil
	}
	// Update our own ParentSHA bookkeeping if we sit directly on trunk
	// or another tracked branch.
	parentTip, err := s.G.RevParse(ctx, meta.Parent)
	if err == nil {
		meta.ParentSHA = parentTip
		_ = s.Store.SetBranch(ctx, current, meta)
	}

	// Now restack every descendant. Git leaves HEAD on the last rebased
	// branch, so return the user to the branch they modified once the
	// stack update completes cleanly.
	if err := restack.New(s.G, s.Store).Restack(ctx, current); err != nil {
		return err
	}
	if err := s.G.Checkout(ctx, current); err != nil {
		return fmt.Errorf("checking out original branch %s: %w", current, err)
	}
	return nil
}

// PausedError exposes the engine's PausedError type to callers so
// command-layer code can detect pauses without importing internal
// packages directly.
type PausedError = restack.PausedError

// Helper used elsewhere when we just want a meta no-op write to make
// tests easier to reason about.
var _ = store.RepoMeta{}
