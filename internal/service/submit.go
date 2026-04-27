package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/stack"
)

// SubmitOptions configures Submit.
type SubmitOptions struct {
	// Stack=true submits the current branch and all its descendants.
	// Stack=false submits only the current branch.
	Stack bool
	// Draft creates new PRs in draft mode. Existing PRs are unaffected.
	Draft bool
	// Body is the body for any newly-created PR. Existing PRs keep
	// their body.
	Body string
}

// SubmitReport summarizes what Submit did per branch.
type SubmitReport struct {
	Pushed  []string
	Created []SubmitPR
	Updated []SubmitPR
	Skipped []SubmitSkip
}

// SubmitPR is a per-branch PR result.
type SubmitPR struct {
	Branch string
	Number int
	URL    string
}

// SubmitSkip explains why a branch was not submitted.
type SubmitSkip struct {
	Branch string
	Reason string
}

// Submit pushes branches and opens or updates pull requests via gh.
// Order: push first (so PRs reference real refs), then PR work in
// parent-before-child order so a base PR exists before its descendant
// retargets it.
func (s *Service) Submit(ctx context.Context, opts SubmitOptions) (SubmitReport, error) {
	r := SubmitReport{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	if err := gh.PreflightCheck(ctx); err != nil {
		return r, err
	}

	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return r, err
	}
	if current == trunk {
		return r, errors.New("refusing to submit trunk")
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}
	if !g.IsTracked(current) {
		return r, fmt.Errorf("branch %q is not tracked; run `sm track` first", current)
	}

	var targets []stack.Branch
	if opts.Stack {
		targets = g.TopoOrderFrom(current)
	} else {
		b, _ := g.Get(current)
		targets = []stack.Branch{b}
	}

	names := make([]string, 0, len(targets))
	for _, t := range targets {
		names = append(names, t.Name)
	}
	s.recordHistory(ctx, "submit", "", names)

	client := gh.New("")

	for _, b := range targets {
		// Refuse to push branches whose ParentSHA is stale: pushing
		// without a clean restack would create a garbled PR.
		if b.Parent != "" && b.Parent != trunk {
			parentTip, err := s.G.RevParse(ctx, b.Parent)
			if err == nil && parentTip != b.ParentSHA {
				r.Skipped = append(r.Skipped, SubmitSkip{
					Branch: b.Name,
					Reason: "needs restack — run `sm restack` first",
				})
				continue
			}
		}

		// Force-with-lease because a previous restack will have
		// rewritten history; plain push would be rejected.
		if err := s.G.Push(ctx, b.Name, true); err != nil {
			return r, fmt.Errorf("pushing %s: %w", b.Name, err)
		}
		r.Pushed = append(r.Pushed, b.Name)
	}

	// Now open / update PRs.
	for _, b := range targets {
		if containsSkip(r.Skipped, b.Name) {
			continue
		}
		base := b.Parent
		if base == "" {
			base = trunk
		}

		existing, ok, err := client.PRForBranch(ctx, b.Name)
		if err != nil {
			return r, err
		}
		if ok {
			// Update base if it drifted.
			if existing.Base != base {
				if err := client.EditPR(ctx, existing.Number, gh.EditPROptions{Base: base}); err != nil {
					return r, fmt.Errorf("retargeting #%d to %s: %w", existing.Number, base, err)
				}
			}
			r.Updated = append(r.Updated, SubmitPR{Branch: b.Name, Number: existing.Number, URL: existing.URL})
			s.persistPR(ctx, b.Name, existing.Number)
			continue
		}

		title := buildPRTitle(b.Name)
		body := opts.Body
		num, err := client.CreatePR(ctx, gh.CreatePROptions{
			Title: title,
			Body:  body,
			Head:  b.Name,
			Base:  base,
			Draft: opts.Draft,
		})
		if err != nil {
			return r, fmt.Errorf("creating PR for %s: %w", b.Name, err)
		}
		r.Created = append(r.Created, SubmitPR{Branch: b.Name, Number: num})
		s.persistPR(ctx, b.Name, num)
	}

	return r, nil
}

// persistPR records the PR number on the branch metadata so future
// `sm log` calls can show it without a gh round-trip.
func (s *Service) persistPR(ctx context.Context, branch string, number int) {
	meta, ok, err := s.Store.GetBranch(ctx, branch)
	if err != nil || !ok {
		return
	}
	meta.PR = number
	_ = s.Store.SetBranch(ctx, branch, meta)
}

// buildPRTitle turns "feat/login-handler" into "feat: login handler".
// Best-effort: keeps the slash group as a conventional-commits scope.
func buildPRTitle(branch string) string {
	parts := strings.SplitN(branch, "/", 2)
	if len(parts) != 2 {
		return strings.ReplaceAll(branch, "-", " ")
	}
	return fmt.Sprintf("%s: %s", parts[0], strings.ReplaceAll(parts[1], "-", " "))
}

func containsSkip(skips []SubmitSkip, branch string) bool {
	for _, s := range skips {
		if s.Branch == branch {
			return true
		}
	}
	return false
}
