package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/stack"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
)

// LogOptions configures Log.
type LogOptions struct {
	// IncludePRStatus, when true, batches a `gh pr list` lookup per
	// branch to decorate nodes with PR state. Skipped if gh isn't
	// available.
	IncludePRStatus bool
	// IncludeChecks adds a CI rollup dot next to each PR pill. Set
	// to false from the cmd layer when --no-checks is passed; falls
	// through silently if gh isn't available.
	IncludeChecks bool
	// IncludeMergeStatus adds the GitHub mergeability glyph next to
	// each PR pill. False when --no-merge-status is passed. Drafts
	// and closed PRs never render a glyph regardless of this flag.
	IncludeMergeStatus bool
}

// Log returns a rendered string showing the stack tree from the trunk
// downward. Caller writes it to stdout.
func (s *Service) Log(ctx context.Context, opts LogOptions) (string, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return "", err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return "", err
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return "", err
	}

	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		// Detached HEAD shouldn't block log output.
		current = ""
	}

	prMap := map[string]gh.PR{}
	statusMap := map[string]gh.PRStatus{}
	if opts.IncludePRStatus {
		client := gh.New("")
		// Best-effort: if gh isn't available the rest of log still works.
		if err := gh.PreflightCheck(ctx); err == nil {
			tracked, _ := s.Store.ListTrackedBranches(ctx)
			if prs, err := client.PRsForBranches(ctx, tracked); err == nil {
				prMap = prs
			}
		}
		statusMap = s.fetchPRStatuses(ctx, prMap, statusFetchOptions{
			wantChecks: opts.IncludeChecks,
			wantMerge:  opts.IncludeMergeStatus,
		})
	}

	var b strings.Builder
	b.WriteString(ui.Banner("stac-man") + "\n\n")

	// Trunk header line — gradient cyan/purple.
	trunkLine := theme.BranchTrunk.Render(trunk)
	if trunk == current {
		trunkLine = theme.BranchCurrent.Render(trunk + "  ← current")
	}
	b.WriteString(trunkLine + "\n")

	roots := g.Roots()
	for i, root := range roots {
		isLast := i == len(roots)-1
		s.renderNode(&b, g, root, "", isLast, current, prMap, statusMap, opts)
	}

	if len(roots) == 0 {
		b.WriteString(ui.Render(theme.Dimmed, "  (no tracked branches yet — `sm create <name>` or `sm track`)") + "\n")
	}

	return b.String(), nil
}

// renderNode draws one branch line plus its subtree. prefix is the
// connector run inherited from the parent's rendering context.
func (s *Service) renderNode(
	b *strings.Builder,
	g *stack.Graph,
	branch stack.Branch,
	prefix string,
	isLast bool,
	current string,
	prMap map[string]gh.PR,
	statusMap map[string]gh.PRStatus,
	opts LogOptions,
) {
	connector := "├─ "
	childPrefix := prefix + "│  "
	if isLast {
		connector = "└─ "
		childPrefix = prefix + "   "
	}

	connectorRendered := ui.Render(theme.Dimmed, prefix+connector)

	name := branch.Name
	style := theme.BranchHealthy
	switch {
	case branch.Name == current:
		style = theme.BranchCurrent
		name = name + "  ← current"
	case s.needsRestack(branch):
		style = theme.BranchNeedsRestack
		name = name + "  (needs restack)"
	}

	suffix := ""
	if pr, ok := prMap[branch.Name]; ok {
		suffix = " " + renderPRPill(pr)
	} else if branch.PR > 0 {
		suffix = ui.Render(theme.Dimmed, fmt.Sprintf(" #%d", branch.PR))
	}

	if status, ok := statusMap[branch.Name]; ok {
		if opts.IncludeChecks {
			if dot := renderCheckDot(status.Checks); dot != "" {
				suffix += " " + dot
			}
		}
		if opts.IncludeMergeStatus && !status.IsDraft {
			if glyph := renderMergeGlyph(status.Mergeable); glyph != "" {
				suffix += " " + glyph
			}
		}
	}

	b.WriteString(connectorRendered + ui.Render(style, name) + suffix + "\n")

	children := g.ChildrenOf(branch.Name)
	for i, child := range children {
		s.renderNode(b, g, child, childPrefix, i == len(children)-1, current, prMap, statusMap, opts)
	}
}

func renderPRPill(pr gh.PR) string {
	label := fmt.Sprintf("#%d", pr.Number)
	switch {
	case pr.IsDraft:
		return ui.Render(theme.PRDraft, label+" draft")
	case pr.State == gh.PRStateMerged:
		return ui.Render(theme.PRMerged, label+" merged")
	case pr.State == gh.PRStateClosed:
		return ui.Render(theme.PRClosed, label+" closed")
	default:
		return ui.Render(theme.PROpen, label+" open")
	}
}

// renderCheckDot returns a single coloured bullet representing the
// CI rollup, or "" when there are no checks at all (a configured-
// less repo would otherwise carry a permanent dim dot for every PR).
func renderCheckDot(rollup gh.CheckRollup) string {
	switch rollup {
	case gh.ChecksPass:
		return ui.Render(theme.CheckOK, "●")
	case gh.ChecksPending:
		return ui.Render(theme.CheckPending, "●")
	case gh.ChecksFail:
		return ui.Render(theme.CheckFail, "●")
	default:
		return ""
	}
}

// renderMergeGlyph returns the per-row mergeability indicator. We
// deliberately use ✓ / ⚠ / ? — distinct shapes from the CI dot — so
// a quick glance at `sm log` separates "CI is green" from "GitHub
// thinks this can merge".
func renderMergeGlyph(m gh.Mergeability) string {
	switch m {
	case gh.MergeMergeable:
		return ui.Render(theme.MergeOK, "✓")
	case gh.MergeConflicting:
		return ui.Render(theme.MergeConflict, "⚠")
	case gh.MergeUnknown:
		return ui.Render(theme.MergeUnknown, "?")
	default:
		return ""
	}
}

// needsRestack reports whether branch.ParentSHA still matches the
// parent's tip. Returns false on read errors so the tree still renders.
func (s *Service) needsRestack(branch stack.Branch) bool {
	if branch.Parent == "" || branch.ParentSHA == "" {
		return false
	}
	ctx := context.Background()
	tip, err := s.G.RevParse(ctx, branch.Parent)
	if err != nil {
		return false
	}
	return tip != branch.ParentSHA
}
