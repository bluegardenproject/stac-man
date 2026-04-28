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

	targets := submissionTargets(g, current, opts.Stack)

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

		derivedTitle, derivedBody := s.derivePRMeta(ctx, b.Name, base)

		existing, ok, err := client.PRForBranch(ctx, b.Name)
		if err != nil {
			return r, err
		}
		if ok {
			edits := gh.EditPROptions{}
			if existing.Base != base {
				edits.Base = base
			}
			// Non-destructive title/body update: only overwrite when
			// the existing values still look like sm's auto-defaults
			// (synthesised title from branch name, empty body) so we
			// never clobber a manually-edited PR description.
			if derivedTitle != "" && existing.Title == buildPRTitle(b.Name) && existing.Title != derivedTitle {
				edits.Title = derivedTitle
			}
			if derivedBody != "" && strings.TrimSpace(existing.Body) == "" {
				edits.Body = derivedBody
			}
			if edits.Title != "" || edits.Body != "" || edits.Base != "" {
				if err := client.EditPR(ctx, existing.Number, edits); err != nil {
					return r, fmt.Errorf("editing #%d: %w", existing.Number, err)
				}
			}
			r.Updated = append(r.Updated, SubmitPR{Branch: b.Name, Number: existing.Number, URL: existing.URL})
			s.persistPR(ctx, b.Name, existing.Number)
			continue
		}

		title := derivedTitle
		if title == "" {
			title = buildPRTitle(b.Name)
		}
		body := opts.Body
		if body == "" {
			body = derivedBody
		}
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

// derivePRMeta builds a PR title and body from the commits unique to
// branch (relative to base). Title is the first commit's subject;
// body is the first commit's body when the branch carries a single
// commit, or a bulleted list of subjects otherwise. Returns empty
// strings on any failure — callers fall back to buildPRTitle and the
// user-supplied --body.
func (s *Service) derivePRMeta(ctx context.Context, branch, base string) (title, body string) {
	commits, err := s.G.LogBetween(ctx, base, branch)
	if err != nil || len(commits) == 0 {
		return "", ""
	}
	title = commits[0].Subject

	if len(commits) == 1 {
		_, b, err := s.G.CommitFullMessage(ctx, commits[0].SHA)
		if err != nil {
			return title, ""
		}
		return title, b
	}

	var sb strings.Builder
	sb.WriteString("Commits in this PR:\n")
	for _, c := range commits {
		fmt.Fprintf(&sb, "- %s\n", c.Subject)
	}
	return title, strings.TrimRight(sb.String(), "\n")
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

// submissionTargets resolves the branches Submit should process.
//
// When wholeStack is true the slice is current + every descendant,
// with any unsubmitted ancestors PREPENDED so a user running
// `sm submit --stack` from the middle (or top) of a fresh stack
// still pushes the whole chain. The walk over ancestors stops at
// the first ancestor that already has a PR — previously-submitted
// or merged stacks below that point are deliberately untouched so
// re-running submit is idempotent and never accidentally re-pushes
// merged history.
//
// When wholeStack is false only the current branch is returned,
// matching the no-flag default.
func submissionTargets(g *stack.Graph, current string, wholeStack bool) []stack.Branch {
	if !wholeStack {
		b, ok := g.Get(current)
		if !ok {
			return nil
		}
		return []stack.Branch{b}
	}

	descendants := g.TopoOrderFrom(current)

	// Ancestors() yields immediate-parent first, then walks up.
	// Collect ancestors with no PR yet; stop at the first one that
	// has been submitted before so we don't reach across stack
	// boundaries into work that's already in review.
	ancestors := g.Ancestors(current)
	var unsubmitted []stack.Branch
	for _, a := range ancestors {
		if a.PR > 0 {
			break
		}
		unsubmitted = append(unsubmitted, a)
	}

	// Reverse unsubmitted so callers see trunk-toward-current order,
	// then concatenate descendants. The push loop later relies on
	// parents-before-children ordering so each PR's base ref already
	// exists on origin by the time we open the PR.
	out := make([]stack.Branch, 0, len(unsubmitted)+len(descendants))
	for i := len(unsubmitted) - 1; i >= 0; i-- {
		out = append(out, unsubmitted[i])
	}
	out = append(out, descendants...)
	return out
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
