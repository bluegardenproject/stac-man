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
	// NoRestack tells Submit to push branches even when their
	// recorded ParentSHA is stale relative to the parent's tip,
	// instead of skipping them with a "needs restack" message. The
	// caller is responsible for understanding that the resulting
	// PR may have a noisy diff; we surface every stale branch in
	// SubmitReport.StaleParentSHA so the warning is unmissable.
	NoRestack bool
	// NoStackTable disables the auto-generated "Stack" block that
	// Submit otherwise injects (or refreshes) at the top of every
	// PR body, fenced by the stac-man sentinels. Useful when the
	// user wants the PR description to stay strictly hand-edited.
	NoStackTable bool
}

// SubmitReport summarizes what Submit did per branch.
type SubmitReport struct {
	Pushed             []string
	SkippedPushes      []string // origin already at local tip — no push issued
	Created            []SubmitPR
	Updated            []SubmitPR
	Skipped            []SubmitSkip
	DivergedStackmates []DivergedBranch // tracked branches with stale origin not in target list
	StaleParentSHA     []string         // pushed via --no-restack despite a stale ParentSHA
}

// DivergedBranch records a tracked branch whose local tip differs
// from origin's, surfaced from plain `sm submit` so the user knows
// the rest of the stack needs a `--stack` push to catch up.
type DivergedBranch struct {
	Branch string
	PR     int // 0 if not yet submitted
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
		// Refuse to push branches whose ParentSHA is stale unless the
		// user opted into --no-restack. classifyStaleParent contains
		// the actual decision so the per-flag behavior is exercised
		// by a focused unit test instead of a full Submit run.
		if b.Parent != "" && b.Parent != trunk {
			parentTip, err := s.G.RevParse(ctx, b.Parent)
			if err == nil && parentTip != b.ParentSHA {
				switch classifyStaleParent(opts.NoRestack) {
				case staleSkip:
					r.Skipped = append(r.Skipped, SubmitSkip{
						Branch: b.Name,
						Reason: "needs restack — run `sm restack` first",
					})
					continue
				case staleWarn:
					r.StaleParentSHA = append(r.StaleParentSHA, b.Name)
				}
			}
		}

		// Idempotent push: skip when origin already has our tip.
		// This is what makes `--stack` always-include-the-whole-
		// chain cheap. We compare local rev-parse to the local
		// tracking ref `origin/<branch>`; we already fetched at
		// the start of any sync, but for `submit` the user may not
		// have. That's fine — a stale tracking ref that says "in
		// sync" while origin moved out from under us is safe to
		// no-op on; the very next `git push --force-with-lease`
		// would catch the divergence anyway.
		if inSync, err := s.G.RemoteMatchesLocal(ctx, b.Name); err == nil && inSync {
			r.SkippedPushes = append(r.SkippedPushes, b.Name)
		} else {
			// Force-with-lease because a previous restack will have
			// rewritten history; plain push would be rejected.
			if err := s.G.Push(ctx, b.Name, true); err != nil {
				return r, fmt.Errorf("pushing %s: %w", b.Name, err)
			}
			r.Pushed = append(r.Pushed, b.Name)
		}
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

	// Plain `sm submit` only refreshed the current branch. If any
	// other tracked branch in the same stack has diverged from
	// origin (typically because `sm sync`'s restack cascade
	// rewrote history after a merge), the user has no signal
	// that those branches will turn into `CONFLICTING` on GitHub
	// the moment a single descendant gets re-pushed. Surface them
	// here so the user can `sm submit --stack` from the bottom
	// without having to discover the gap by hitting it.
	if !opts.Stack {
		r.DivergedStackmates = s.divergedStackmates(ctx, g, current, targets)
	}

	// Stack-table pass runs AFTER every PR exists, because each PR's
	// body needs to reference siblings whose numbers were unknown
	// when their own create/update call ran. Skips silently when
	// the user opted out via --no-stack-table.
	if !opts.NoStackTable {
		s.applyStackTables(ctx, client, g, targets, &r)
	}

	// Submit can change CI state (push triggers a fresh run) and
	// mergeability (retargeted base, new tip on origin). The cache
	// would otherwise serve the pre-push status to the very next
	// `sm log`, defeating the round-trip we just paid for.
	if gitDir, err := s.G.GitDir(ctx); err == nil {
		_ = gh.InvalidateChecksCache(gitDir)
	}

	return r, nil
}

// applyStackTables walks every target with a known PR number and
// rewrites its PR body so the stac-man-fenced "Stack" block reflects
// the current chain. Idempotent: a branch whose body already carries
// the same block is left alone (no needless `gh pr edit` round-trip),
// and re-runs of `sm submit` simply replace the block in-place.
//
// Each target costs one `gh pr view` (for the latest body) plus an
// optional `gh pr edit` when the body actually changes. Errors are
// best-effort — a flaky gh round-trip shouldn't fail the whole
// submit when the PRs themselves were created/updated successfully.
func (s *Service) applyStackTables(ctx context.Context, client *gh.Client, g *stack.Graph, targets []stack.Branch, r *SubmitReport) {
	prByBranch := buildPRMap(g, *r)
	if len(prByBranch) == 0 {
		return
	}
	for _, t := range targets {
		if containsSkip(r.Skipped, t.Name) {
			continue
		}
		num, ok := prByBranch[t.Name]
		if !ok || num == 0 {
			continue
		}
		chain := stackChainForBranch(g, t.Name)
		table := renderStackTable(chain, t.Name, prByBranch)
		if table == "" {
			continue
		}
		// Re-fetch so a manual edit between the create/update call
		// above and this pass is preserved outside the sentinels.
		existing, ok, err := client.PRForBranch(ctx, t.Name)
		if err != nil || !ok {
			continue
		}
		newBody := injectStackTable(existing.Body, table)
		if newBody == existing.Body {
			continue
		}
		_ = client.EditPR(ctx, num, gh.EditPROptions{Body: newBody})
	}
}

// buildPRMap merges the persisted PR numbers from the stack graph
// with the freshly-allocated numbers from this submit run. Created
// PRs land first, followed by Updated, so a re-run that recreates a
// branch (rare but possible) overrides the stale stored number.
func buildPRMap(g *stack.Graph, r SubmitReport) map[string]int {
	out := map[string]int{}
	for _, b := range g.Branches() {
		if b.PR > 0 {
			out[b.Name] = b.PR
		}
	}
	for _, p := range r.Updated {
		if p.Number > 0 {
			out[p.Branch] = p.Number
		}
	}
	for _, p := range r.Created {
		if p.Number > 0 {
			out[p.Branch] = p.Number
		}
	}
	return out
}

// divergedStackmates returns tracked branches whose local tip
// differs from origin's local tracking ref, excluding (a) trunk and
// (b) anything in `targets` (which were either pushed or
// intentionally skipped). Best-effort — a branch with no remote
// tracking ref or a transient git error simply doesn't appear in
// the list rather than failing the whole submit.
func (s *Service) divergedStackmates(ctx context.Context, g *stack.Graph, current string, targets []stack.Branch) []DivergedBranch {
	inTargets := make(map[string]bool, len(targets))
	for _, t := range targets {
		inTargets[t.Name] = true
	}
	var out []DivergedBranch
	for _, b := range g.Branches() {
		if b.Name == g.Trunk || inTargets[b.Name] {
			continue
		}
		inSync, err := s.G.RemoteMatchesLocal(ctx, b.Name)
		if err != nil || inSync {
			continue
		}
		out = append(out, DivergedBranch{Branch: b.Name, PR: b.PR})
	}
	return out
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

	// `--stack` targets the entire chain in trunk-toward-leaf order:
	// every ancestor of `current`, then `current` itself, then every
	// descendant. We deliberately do NOT filter ancestors by PR
	// state — earlier versions stopped at the first PR'd ancestor
	// (B7) to keep re-runs cheap, but that left mid-stack rewrites
	// stranded on origin: after a merge of an ancestor, `sm sync`
	// rebases every descendant locally and the recorded PR'd
	// branches above `current` end up out of sync with origin,
	// silently breaking mergeability on GitHub.
	//
	// Idempotency now lives in the push loop instead — it skips
	// `git push` for branches whose origin ref already matches the
	// local tip. That makes "always include the whole stack" cheap
	// to re-run while ensuring nothing gets left stale, which is
	// what users coming from Graphite expect from `gt submit --stack`.
	ancestors := g.Ancestors(current) // immediate-parent first
	descendants := g.TopoOrderFrom(current)

	out := make([]stack.Branch, 0, len(ancestors)+len(descendants))
	for i := len(ancestors) - 1; i >= 0; i-- {
		out = append(out, ancestors[i])
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

// staleParentAction is the outcome of the parent-SHA freshness check
// for a single submit target. Exists as a typed value so the no-flag
// vs --no-restack split has one obvious place in the codebase.
type staleParentAction int

const (
	// staleSkip removes the branch from the push and PR-edit loop and
	// records a "needs restack" entry in SubmitReport.Skipped. This is
	// the historical default — users had to restack before submitting
	// any branch with a stale parent.
	staleSkip staleParentAction = iota
	// staleWarn lets the push proceed but records the branch in
	// SubmitReport.StaleParentSHA so the user has an unmissable signal
	// that the resulting PR may show parent commits in its diff.
	staleWarn
)

// classifyStaleParent returns the action Submit should take when a
// target's recorded ParentSHA differs from the parent's tip.
// noRestack is opts.NoRestack: setting it (the new --no-restack flag)
// flips the default skip into a non-fatal warning + push.
func classifyStaleParent(noRestack bool) staleParentAction {
	if noRestack {
		return staleWarn
	}
	return staleSkip
}
