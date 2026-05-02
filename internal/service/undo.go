package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/history"
	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// snapshot captures the current state of `branch` for a future undo.
// Missing branches and untracked branches both record valid Snapshots
// (with Existed/Tracked flags) so undo knows whether to delete or
// untrack them on rollback.
func (s *Service) snapshot(ctx context.Context, branch string) history.Snapshot {
	snap := history.Snapshot{Branch: branch}

	if exists, _ := s.G.BranchExists(ctx, branch); exists {
		snap.Existed = true
		if tip, err := s.G.RevParse(ctx, branch); err == nil {
			snap.Tip = tip
		}
	}
	if meta, ok, err := s.Store.GetBranch(ctx, branch); err == nil && ok {
		snap.Tracked = true
		snap.ParentName = meta.Parent
		snap.ParentSHA = meta.ParentSHA
		snap.PR = meta.PR
	}
	return snap
}

// recordHistory writes a history entry for the upcoming op. branches
// is the set the op may touch — duplicates are fine, the snapshot key
// is the branch name. Failures are logged-and-swallowed: undo is
// best-effort and we don't want a flaky filesystem to abort a
// successful command.
func (s *Service) recordHistory(ctx context.Context, op, notes string, branches []string) {
	gitDir, err := s.G.GitDir(ctx)
	if err != nil {
		return
	}
	head, _ := s.G.CurrentBranch(ctx)

	seen := map[string]bool{}
	before := map[string]history.Snapshot{}
	for _, name := range branches {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		before[name] = s.snapshot(ctx, name)
	}
	entry := history.Entry{
		Op:     op,
		Notes:  notes,
		HEAD:   head,
		Before: before,
	}
	_ = history.Append(gitDir, entry)
}

// UndoResult is what cmd/undo.go renders.
type UndoResult struct {
	Op       string
	Notes    string
	Branches []string
	DryRun   bool
}

// Undo pops the most recent history entry and restores every branch
// in its Before snapshot. Tip restoration uses `git update-ref`; for
// branches that were created by the op we delete them; for branches
// untracked by the op we re-track them with the snapshot metadata.
//
// Refuses while a restack is paused — the user must finish or abort
// the in-flight restack first because update-ref'ing branches in the
// middle of a rebase would corrupt the resume state.
func (s *Service) Undo(ctx context.Context, dryRun bool) (UndoResult, error) {
	r := UndoResult{DryRun: dryRun}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	gitDir, err := s.G.GitDir(ctx)
	if err != nil {
		return r, err
	}

	if inProgress, _ := s.G.RebaseInProgress(ctx); inProgress {
		return r, errors.New("rebase in progress; finish with `sm continue` or `sm abort` before undoing")
	}

	if dryRun {
		entry, ok, err := history.Peek(gitDir)
		if err != nil {
			return r, err
		}
		if !ok {
			return r, errors.New("nothing to undo")
		}
		return undoResultFrom(entry, true), nil
	}

	entry, ok, err := history.Pop(gitDir)
	if err != nil {
		return r, err
	}
	if !ok {
		return r, errors.New("nothing to undo")
	}

	// Track branches whose tips we move so we can restack them after.
	moved := []string{}
	for name, snap := range entry.Before {
		if !snap.Existed {
			// Op created this branch — undo by deleting and untracking.
			_ = s.Store.UnsetBranch(ctx, name)
			if exists, _ := s.G.BranchExists(ctx, name); exists {
				if err := s.G.DeleteBranch(ctx, name, true); err != nil {
					return r, fmt.Errorf("undo: deleting %s: %w", name, err)
				}
			}
			continue
		}

		// Branch existed pre-op. Restore tip if it has changed.
		if snap.Tip != "" {
			if currentTip, err := s.G.RevParse(ctx, name); err == nil && currentTip != snap.Tip {
				if err := s.G.UpdateRef(ctx, name, snap.Tip); err != nil {
					return r, fmt.Errorf("undo: restoring %s tip: %w", name, err)
				}
				moved = append(moved, name)
			}
		}

		// Restore tracking metadata.
		if snap.Tracked {
			if err := s.Store.SetBranch(ctx, name, branchMetaFromSnap(snap)); err != nil {
				return r, fmt.Errorf("undo: restoring tracking for %s: %w", name, err)
			}
		} else {
			if err := s.Store.UnsetBranch(ctx, name); err != nil {
				return r, fmt.Errorf("undo: untracking %s: %w", name, err)
			}
		}
	}

	// Hop back to the prior HEAD branch if it still exists.
	if entry.HEAD != "" {
		if exists, _ := s.G.BranchExists(ctx, entry.HEAD); exists {
			_ = s.G.Checkout(ctx, entry.HEAD)
		}
	}

	// Restack any survivor whose tip we moved, so descendants honor
	// the restored history.
	for _, name := range moved {
		if _, ok, _ := s.Store.GetBranch(ctx, name); !ok {
			continue
		}
		if err := restack.New(s.G, s.Store).Restack(ctx, name); err != nil {
			// Stop reporting after the first paused restack — user
			// has to resolve before continuing the undo's cascade.
			return undoResultFrom(entry, false), err
		}
	}

	return undoResultFrom(entry, false), nil
}

func undoResultFrom(e history.Entry, dryRun bool) UndoResult {
	branches := make([]string, 0, len(e.Before))
	for name := range e.Before {
		branches = append(branches, name)
	}
	return UndoResult{
		Op:       e.Op,
		Notes:    e.Notes,
		Branches: branches,
		DryRun:   dryRun,
	}
}

// branchMetaFromSnap rebuilds a store.BranchMeta from a Snapshot.
func branchMetaFromSnap(snap history.Snapshot) store.BranchMeta {
	return store.BranchMeta{
		Parent:    snap.ParentName,
		ParentSHA: snap.ParentSHA,
		PR:        snap.PR,
	}
}
