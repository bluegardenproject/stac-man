package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/store"
)

// CreateOptions configures Create.
type CreateOptions struct {
	// Name of the new branch.
	Name string
	// CommitMessage, if non-empty, triggers a commit on the new
	// branch immediately after checkout. Requires StageAll=true or
	// pre-staged changes.
	CommitMessage string
	// StageAll stages tracked-but-modified files before committing,
	// matching `git commit -a` semantics. Untracked files are NOT
	// included unless IncludeUntracked is also true.
	StageAll bool
	// IncludeUntracked, in combination with StageAll, also stages
	// new (untracked) files. Off by default to avoid silently
	// absorbing unrelated work-in-progress.
	IncludeUntracked bool
}

// Create branches off HEAD, sets the parent metadata, and optionally
// commits any staged or to-be-staged changes. The new branch's parent
// is the branch HEAD was on when Create was called (or the trunk if
// HEAD was on the trunk).
func (s *Service) Create(ctx context.Context, opts CreateOptions) error {
	if opts.Name == "" {
		return errors.New("branch name is required")
	}

	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return err
	}

	parent, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("reading current branch: %w", err)
	}

	// If we're not committing as part of create, refuse to create on
	// a dirty tree — the resulting branch would carry over staged
	// changes silently.
	if opts.CommitMessage == "" && !opts.StageAll {
		clean, err := s.G.IsClean(ctx)
		if err != nil {
			return err
		}
		if !clean {
			return errors.New("working tree has uncommitted changes; commit them first or pass -m to commit on the new branch")
		}
	}

	if exists, err := s.G.BranchExists(ctx, opts.Name); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("branch %q already exists", opts.Name)
	}

	s.recordHistory(ctx, "create", opts.Name, []string{opts.Name, parent})

	if err := s.G.CheckoutNew(ctx, opts.Name); err != nil {
		return fmt.Errorf("creating branch: %w", err)
	}

	if opts.CommitMessage != "" || opts.StageAll {
		if opts.StageAll {
			if err := stageWorkingTree(ctx, s.G, opts.IncludeUntracked); err != nil {
				return err
			}
		}
		if opts.CommitMessage != "" {
			if err := s.G.Commit(ctx, opts.CommitMessage, false, false); err != nil {
				return fmt.Errorf("committing: %w", err)
			}
		}
	}

	parentSHA, err := s.G.RevParse(ctx, parent)
	if err != nil {
		return fmt.Errorf("resolving parent SHA: %w", err)
	}

	meta := store.BranchMeta{Parent: parent, ParentSHA: parentSHA}
	if err := s.Store.SetBranch(ctx, opts.Name, meta); err != nil {
		return fmt.Errorf("recording stack metadata: %w", err)
	}

	// If the parent is the trunk and the trunk itself isn't tracked
	// yet (it shouldn't be, by convention), there's nothing to do.
	// Tracking exists only for non-trunk branches.
	_ = trunk
	return nil
}
