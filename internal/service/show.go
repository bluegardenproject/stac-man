package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/stack"
)

// BranchView is the rich, render-agnostic snapshot returned by Show.
// JSON tags drive `sm show --json` so the same struct doubles as the
// machine-readable contract for AI agents and scripts.
type BranchView struct {
	Branch       string       `json:"branch"`
	Trunk        string       `json:"trunk"`
	Tracked      bool         `json:"tracked"`
	Parent       string       `json:"parent,omitempty"`
	ParentSHA    string       `json:"parentSHA,omitempty"`
	Tip          string       `json:"tip,omitempty"`
	NeedsRestack bool         `json:"needsRestack"`
	Children     []string     `json:"children,omitempty"`
	Ancestors    []string     `json:"ancestors,omitempty"` // immediate parent first, trunk excluded
	AheadParent  int          `json:"aheadParent"`
	BehindParent int          `json:"behindParent"`
	AheadTrunk   int          `json:"aheadTrunk"`
	BehindTrunk  int          `json:"behindTrunk"`
	PR           *PRView      `json:"pr,omitempty"`
	Commits      []CommitView `json:"commits,omitempty"`
}

// PRView is the subset of PR fields stac-man surfaces in `sm show`
// and `sm log --json`. The two commands share this type so a single
// JSON parser handles both. Checks and Mergeable are populated by
// `sm log` when the gh round-trip provides them; `sm show` leaves
// them at their zero values (CheckRollup="" and Mergeability="").
// Both fields are omitempty so consumers can distinguish "not
// computed" from any of the meaningful states.
type PRView struct {
	Number    int             `json:"number"`
	State     string          `json:"state"`
	URL       string          `json:"url"`
	Draft     bool            `json:"draft"`
	Title     string          `json:"title,omitempty"`
	Checks    gh.CheckRollup  `json:"checks,omitempty"`
	Mergeable gh.Mergeability `json:"mergeable,omitempty"`
}

// CommitView is one commit unique to the branch (vs parent).
type CommitView struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// Show returns a BranchView for the named branch (or current when
// branch is empty). Uses the gh CLI opportunistically — if `gh` is
// unauthenticated or absent, the PR field is left nil.
func (s *Service) Show(ctx context.Context, branch string) (*BranchView, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return nil, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return nil, err
	}
	if branch == "" {
		branch, err = s.G.CurrentBranch(ctx)
		if err != nil {
			return nil, err
		}
	}
	if exists, err := s.G.BranchExists(ctx, branch); err != nil {
		return nil, err
	} else if !exists {
		return nil, fmt.Errorf("branch %q does not exist", branch)
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return nil, err
	}

	view := &BranchView{Branch: branch, Trunk: trunk}

	tip, err := s.G.RevParse(ctx, branch)
	if err == nil {
		view.Tip = tip
	}

	b, tracked := g.Get(branch)
	view.Tracked = tracked
	parent := trunk
	if tracked {
		view.Parent = b.Parent
		view.ParentSHA = b.ParentSHA
		if b.Parent != "" {
			parent = b.Parent
		}
		if currentParent, perr := s.G.RevParse(ctx, parent); perr == nil {
			view.NeedsRestack = b.ParentSHA != "" && currentParent != b.ParentSHA
		}
		for _, child := range g.ChildrenOf(branch) {
			view.Children = append(view.Children, child.Name)
		}
		for _, anc := range g.Ancestors(branch) {
			view.Ancestors = append(view.Ancestors, anc.Name)
		}
	}

	view.AheadParent, view.BehindParent = countAheadBehind(ctx, s.G, branch, parent)
	view.AheadTrunk, view.BehindTrunk = countAheadBehind(ctx, s.G, branch, trunk)

	commits, err := s.G.LogBetween(ctx, parent, branch)
	if err == nil {
		for _, c := range commits {
			view.Commits = append(view.Commits, CommitView{SHA: c.SHA, Subject: c.Subject})
		}
	}

	if pr, perr := lookupPR(ctx, branch); perr == nil && pr != nil {
		view.PR = pr
	}

	return view, nil
}

// countAheadBehind returns commits-ahead and commits-behind of branch
// vs base, suppressing errors. A swallowed error yields zero counts —
// fine for a read-only view.
func countAheadBehind(ctx context.Context, g *git.Client, branch, base string) (ahead, behind int) {
	if branch == "" || base == "" || branch == base {
		return 0, 0
	}
	a, err := g.CountCommitsAhead(ctx, branch, base)
	if err == nil {
		ahead = a
	}
	b, err := g.CountCommitsAhead(ctx, base, branch)
	if err == nil {
		behind = b
	}
	return ahead, behind
}

// lookupPR returns a PRView if gh can give us one, or nil + nil error
// if there's no PR. Auth/install errors are swallowed so Show stays
// useful offline.
func lookupPR(ctx context.Context, branch string) (*PRView, error) {
	if err := gh.PreflightCheck(ctx); err != nil {
		return nil, nil //nolint: returns nil view
	}
	pr, ok, err := gh.New("").PRForBranch(ctx, branch)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &PRView{
		Number: pr.Number,
		State:  string(pr.State),
		URL:    pr.URL,
		Draft:  pr.IsDraft,
		Title:  pr.Title,
	}, nil
}

// ErrNoBranch is returned when Show is asked about a branch that
// doesn't exist locally.
var ErrNoBranch = errors.New("branch does not exist")
