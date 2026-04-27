package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/restack"
	"github.com/philipptpunkt/stac-man/internal/store"
)

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
	// StageAll stages all unstaged changes before committing/amending.
	StageAll bool
	// Message is the commit message. Required when Commit=true and
	// Amend=false; optional otherwise.
	Message string
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

	if opts.StageAll {
		if err := s.G.AddAll(ctx); err != nil {
			return fmt.Errorf("staging changes: %w", err)
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

	// Now restack every descendant.
	return restack.New(s.G, s.Store).Restack(ctx, current)
}

// PausedError exposes the engine's PausedError type to callers so
// command-layer code can detect pauses without importing internal
// packages directly.
type PausedError = restack.PausedError

// Helper used elsewhere when we just want a meta no-op write to make
// tests easier to reason about.
var _ = store.RepoMeta{}
