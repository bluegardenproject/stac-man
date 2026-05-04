package restack

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// Git is the slice of *git.Client the engine relies on. Defining it
// here lets unit tests inject a fake without depending on the full
// git package.
type Git interface {
	GitDir(ctx context.Context) (string, error)
	CurrentBranch(ctx context.Context) (string, error)
	RevParse(ctx context.Context, ref string) (string, error)
	Checkout(ctx context.Context, branch string) error
	Rebase(ctx context.Context, onto, upstream, branch string) error
	RebaseInProgress(ctx context.Context) (bool, error)
	RebaseContinue(ctx context.Context) error
	RebaseAbort(ctx context.Context) error
	// ConflictPaths is consulted by Engine.Paused to surface the
	// list of unmerged files alongside the on-disk pending queue.
	ConflictPaths(ctx context.Context) ([]string, error)
}

// Engine orchestrates the rebase walk for `sm restack` / `sm sync`.
// It is stateless across invocations — resume state lives on disk via
// the State helpers in state.go.
type Engine struct {
	G     Git
	Store store.Store
}

// New constructs an Engine.
func New(g Git, s store.Store) *Engine {
	return &Engine{G: g, Store: s}
}

// PausedError is returned when a rebase conflicts mid-walk. The
// presence of this error type tells command-layer code to print the
// "resolve, then `sm continue`" guidance rather than a generic stack
// trace.
type PausedError struct {
	Branch  string
	Wrapped error
}

func (e *PausedError) Error() string {
	if e.Wrapped == nil {
		return fmt.Sprintf("rebase paused on %s", e.Branch)
	}
	return fmt.Sprintf("rebase paused on %s: %v", e.Branch, e.Wrapped)
}
func (e *PausedError) Unwrap() error { return e.Wrapped }

// Restack rebases startBranch and every tracked descendant onto its
// parent's current tip. On conflict it persists resume state and
// returns a *PausedError.
func (e *Engine) Restack(ctx context.Context, startBranch string) error {
	g, err := stack.Load(ctx, e.Store)
	if err != nil {
		return err
	}
	if !g.IsTracked(startBranch) {
		return fmt.Errorf("branch %q is not tracked", startBranch)
	}

	// Build the topo-ordered queue: startBranch + descendants.
	chain := g.TopoOrderFrom(startBranch)

	gitDir, err := e.G.GitDir(ctx)
	if err != nil {
		return err
	}

	startTip, err := e.G.RevParse(ctx, startBranch)
	if err != nil {
		return err
	}

	st := &State{
		Origin:      "restack",
		SavedBranch: startBranch,
		SavedTip:    startTip,
		Pending:     []PendingBranch{},
	}
	// willMove tracks branches we've decided to rebase. A descendant
	// whose parent is in this set ALWAYS needs to rebase, regardless
	// of the on-disk parent SHA, because the parent's tip will be
	// rewritten by an earlier step.
	willMove := map[string]bool{}
	for _, b := range chain {
		needsRebase := willMove[b.Parent]
		if !needsRebase {
			oldParentSHA := b.ParentSHA
			currentParentSHA, err := e.G.RevParse(ctx, b.Parent)
			if err != nil {
				return fmt.Errorf("resolving %s tip: %w", b.Parent, err)
			}
			needsRebase = oldParentSHA != currentParentSHA
		}
		if !needsRebase {
			continue
		}
		st.Pending = append(st.Pending, PendingBranch{
			Name:      b.Name,
			OldParent: b.Parent,
			NewParent: b.Parent,
			OldSHA:    b.ParentSHA,
		})
		willMove[b.Name] = true
	}

	if len(st.Pending) == 0 {
		return nil
	}

	return e.runQueue(ctx, gitDir, st)
}

// Continue resumes from a previously paused state, after the user has
// resolved conflicts and run `git rebase --continue`-equivalent
// staging. We let git finish the in-flight rebase first, then process
// the remaining queue.
func (e *Engine) Continue(ctx context.Context) error {
	gitDir, err := e.G.GitDir(ctx)
	if err != nil {
		return err
	}
	st, ok, err := Load(gitDir)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("no paused stac-man operation to continue")
	}

	if inProgress, err := e.G.RebaseInProgress(ctx); err != nil {
		return err
	} else if inProgress {
		if err := e.G.RebaseContinue(ctx); err != nil {
			return &PausedError{Branch: currentName(st), Wrapped: err}
		}
	}

	// The branch at the head of Pending just finished — record its new
	// parent SHA before moving on.
	if len(st.Pending) > 0 {
		head := st.Pending[0]
		if err := e.recordSuccess(ctx, head); err != nil {
			return err
		}
		st.Pending = st.Pending[1:]
	}

	return e.runQueue(ctx, gitDir, st)
}

// Abort discards the in-flight rebase, restores the original branch,
// and clears the resume state.
func (e *Engine) Abort(ctx context.Context) error {
	gitDir, err := e.G.GitDir(ctx)
	if err != nil {
		return err
	}
	st, ok, err := Load(gitDir)
	if err != nil {
		return err
	}
	if inProgress, err := e.G.RebaseInProgress(ctx); err != nil {
		return err
	} else if inProgress {
		if err := e.G.RebaseAbort(ctx); err != nil {
			return err
		}
	}
	if ok && st.SavedBranch != "" {
		_ = e.G.Checkout(ctx, st.SavedBranch)
	}
	return Clear(gitDir)
}

// runQueue replays the pending rebases, persisting state at each step
// so a Ctrl+C or conflict leaves a recoverable resume point. Empties
// the queue on success.
func (e *Engine) runQueue(ctx context.Context, gitDir string, st *State) error {
	for len(st.Pending) > 0 {
		head := st.Pending[0]

		// Persist BEFORE running the rebase: if the rebase conflicts,
		// the state we just wrote is what `sm continue` will read.
		if err := Save(gitDir, st); err != nil {
			return err
		}

		// Re-resolve the new parent SHA each step — earlier steps in
		// the queue may have moved a parent further along.
		newParentSHA, err := e.G.RevParse(ctx, head.NewParent)
		if err != nil {
			return err
		}
		if err := e.G.Rebase(ctx, newParentSHA, head.OldSHA, head.Name); err != nil {
			// Leave state intact for `sm continue`.
			return &PausedError{Branch: head.Name, Wrapped: err}
		}

		if err := e.recordSuccess(ctx, head); err != nil {
			return err
		}
		st.Pending = st.Pending[1:]
	}
	return Clear(gitDir)
}

// recordSuccess updates branch.<name>.stac-man-parent-sha to the
// parent's tip after a clean rebase.
func (e *Engine) recordSuccess(ctx context.Context, head PendingBranch) error {
	tip, err := e.G.RevParse(ctx, head.NewParent)
	if err != nil {
		return err
	}
	meta, _, err := e.Store.GetBranch(ctx, head.Name)
	if err != nil {
		return err
	}
	meta.Parent = head.NewParent
	meta.ParentSHA = tip
	return e.Store.SetBranch(ctx, head.Name, meta)
}

func currentName(st *State) string {
	if st == nil || len(st.Pending) == 0 {
		return ""
	}
	return st.Pending[0].Name
}
