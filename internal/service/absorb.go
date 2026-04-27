package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/restack"
	"github.com/philipptpunkt/stac-man/internal/stack"
)

// AbsorbOptions configures Absorb.
type AbsorbOptions struct {
	// Base, if non-empty, overrides the default base. The default
	// base is the current branch's lowest tracked ancestor (or trunk
	// if there is none). Pass a branch name; SHAs are also accepted
	// but discouraged because they can't be re-resolved later.
	Base string
}

// AbsorbResult summarizes what Absorb did so the cmd layer can render
// a tidy line or two.
type AbsorbResult struct {
	Branch    string
	Base      string
	Restacked bool
}

// Absorb runs `git-absorb` to route uncommitted hunks back into the
// right ancestor commits, then triggers a restack so descendants pick
// up the rewritten ancestors. Refuses cleanly if `git-absorb` isn't
// installed or the working tree has nothing to absorb.
func (s *Service) Absorb(ctx context.Context, opts AbsorbOptions) (AbsorbResult, error) {
	r := AbsorbResult{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	if !git.HasGitAbsorb() {
		return r, errors.New("git-absorb is not installed; install from https://github.com/tummychow/git-absorb (e.g. `brew install git-absorb`) and re-run")
	}

	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return r, err
	}
	if current == trunk {
		return r, fmt.Errorf("refusing to absorb on trunk %q", trunk)
	}
	r.Branch = current

	// Refuse on a fully-clean tree: git-absorb is a no-op there and
	// would leave the user wondering whether anything happened.
	if clean, err := s.G.IsClean(ctx); err != nil {
		return r, err
	} else if clean {
		return r, errors.New("nothing to absorb: working tree has no staged or unstaged changes")
	}

	base, err := s.resolveAbsorbBase(ctx, current, trunk, opts.Base)
	if err != nil {
		return r, err
	}
	r.Base = base

	baseSHA, err := s.G.RevParse(ctx, base)
	if err != nil {
		return r, fmt.Errorf("resolving %s: %w", base, err)
	}

	s.recordHistory(ctx, "absorb", current, branchAndDescendants(ctx, s, current))

	if err := s.G.Absorb(ctx, baseSHA); err != nil {
		return r, fmt.Errorf("git-absorb failed: %w", err)
	}

	// Cascade: any descendant of `current` whose parent has just been
	// rewritten needs to rebase. The restack engine keys off
	// ParentSHA drift, so this Just Works.
	if err := restack.New(s.G, s.Store).Restack(ctx, current); err != nil {
		return r, err
	}
	r.Restacked = true
	return r, nil
}

// resolveAbsorbBase returns the lowest tracked ancestor of branch
// (i.e. the deepest non-trunk ancestor that's still tracked), or the
// trunk if branch sits directly on trunk. An explicit override wins
// over the auto-resolved value but must point at a real branch.
func (s *Service) resolveAbsorbBase(ctx context.Context, branch, trunk, override string) (string, error) {
	if override != "" {
		if override == trunk {
			return trunk, nil
		}
		if exists, err := s.G.BranchExists(ctx, override); err != nil {
			return "", err
		} else if !exists {
			return "", fmt.Errorf("base %q does not exist", override)
		}
		return override, nil
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return "", err
	}
	cur, ok := g.Get(branch)
	if !ok {
		// Branch isn't tracked: absorb is still useful, but we must
		// fall back to merge-base against trunk.
		return trunk, nil
	}
	if cur.Parent == "" || cur.Parent == trunk {
		return trunk, nil
	}
	return cur.Parent, nil
}
