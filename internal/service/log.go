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

// logData is the structured intermediate Log builds before rendering.
// It captures the full stack graph plus per-branch PR / status
// snapshots so the renderer is a pure formatter — every fetch,
// rev-parse, and gh round-trip happens during buildLogData and the
// rest of the pipeline reads from this immutable shape.
//
// Keeping this type unexported in step 1 lets us iterate on the
// internal contract; step 2 promotes it to the public LogResult that
// `sm log --json` emits.
type logData struct {
	Trunk   string
	Current string
	Roots   []*logBranchNode
}

// logBranchNode is one node in the rendered tree. Children are built
// recursively so the renderer can walk them with the same is-last /
// connector logic as before; the PR and Status pointers are nil-able
// to distinguish "no PR" from "PR present but zero-valued".
type logBranchNode struct {
	Branch       stack.Branch
	NeedsRestack bool
	PR           *gh.PR
	Status       *gh.PRStatus
	Children     []*logBranchNode
}

// Log returns a rendered string showing the stack tree from the trunk
// downward. Caller writes it to stdout.
func (s *Service) Log(ctx context.Context, opts LogOptions) (string, error) {
	d, err := s.buildLogData(ctx, opts)
	if err != nil {
		return "", err
	}
	return renderLogTree(d, opts), nil
}

// buildLogData runs every side-effectful step Log needs (repo + trunk
// preflight, graph load, current-branch read, PR + status fetches,
// per-branch needs-restack computation) and returns a fully-populated
// logData. It is the single seam where I/O happens for `sm log` and
// (in step 2) the new `--json` / `--porcelain` formatters.
func (s *Service) buildLogData(ctx context.Context, opts LogOptions) (*logData, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return nil, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return nil, err
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return nil, err
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

	needsRestack := s.computeNeedsRestackMap(ctx, g)

	return &logData{
		Trunk:   trunk,
		Current: current,
		Roots:   buildLogTree(g, prMap, statusMap, needsRestack),
	}, nil
}

// buildLogTree is the pure shape-builder. Given a stack graph and the
// per-branch lookup maps, it produces the recursive node tree the
// renderer walks. Pulled out from buildLogData so unit tests can pin
// the exact tree shape produced for a given graph without spinning
// up a real Service / git fixture.
func buildLogTree(g *stack.Graph, prMap map[string]gh.PR, statusMap map[string]gh.PRStatus, needsRestack map[string]bool) []*logBranchNode {
	roots := g.Roots()
	out := make([]*logBranchNode, 0, len(roots))
	for _, r := range roots {
		out = append(out, buildLogNode(g, r, prMap, statusMap, needsRestack))
	}
	return out
}

func buildLogNode(g *stack.Graph, b stack.Branch, prMap map[string]gh.PR, statusMap map[string]gh.PRStatus, needsRestack map[string]bool) *logBranchNode {
	n := &logBranchNode{
		Branch:       b,
		NeedsRestack: needsRestack[b.Name],
	}
	if pr, ok := prMap[b.Name]; ok {
		prCopy := pr
		n.PR = &prCopy
	}
	if st, ok := statusMap[b.Name]; ok {
		stCopy := st
		n.Status = &stCopy
	}
	for _, c := range g.ChildrenOf(b.Name) {
		n.Children = append(n.Children, buildLogNode(g, c, prMap, statusMap, needsRestack))
	}
	return n
}

// computeNeedsRestackMap pre-computes the needs-restack flag for
// every tracked branch in g. Walking the graph once here keeps the
// rev-parse cost bounded and lets the pure renderer / formatters
// read the result from a map instead of calling back into git.
func (s *Service) computeNeedsRestackMap(ctx context.Context, g *stack.Graph) map[string]bool {
	out := map[string]bool{}
	for _, b := range g.Branches() {
		if s.needsRestack(b) {
			out[b.Name] = true
		}
	}
	return out
}

// needsRestack reports whether branch.ParentSHA still matches the
// parent's tip. Returns false on read errors so the tree still renders
// (and so nav.go's selector still works when git is partially broken).
// Kept as a method because nav.go consumes it per-branch; log.go uses
// computeNeedsRestackMap to batch the rev-parses up front.
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

// renderLogTree is the pure formatter. It consumes a fully-populated
// logData and emits the synthwave banner + tree the user sees today.
// No I/O, no Service access — every input it needs is already on d
// or opts. This is what the step-2 formatters will sit beside as
// peers (renderLogJSON, renderLogPorcelain).
func renderLogTree(d *logData, opts LogOptions) string {
	var b strings.Builder
	b.WriteString(ui.Banner("stac-man") + "\n\n")

	// Trunk header line — gradient cyan/purple.
	trunkLine := theme.BranchTrunk.Render(d.Trunk)
	if d.Trunk == d.Current {
		trunkLine = theme.BranchCurrent.Render(d.Trunk + "  ← current")
	}
	b.WriteString(trunkLine + "\n")

	for i, root := range d.Roots {
		isLast := i == len(d.Roots)-1
		renderNode(&b, root, "", isLast, d.Current, opts)
	}

	if len(d.Roots) == 0 {
		b.WriteString(ui.Render(theme.Dimmed, "  (no tracked branches yet — `sm create <name>` or `sm track`)") + "\n")
	}

	return b.String()
}

// renderNode draws one branch line plus its subtree. prefix is the
// connector run inherited from the parent's rendering context.
func renderNode(b *strings.Builder, node *logBranchNode, prefix string, isLast bool, current string, opts LogOptions) {
	connector := "├─ "
	childPrefix := prefix + "│  "
	if isLast {
		connector = "└─ "
		childPrefix = prefix + "   "
	}

	connectorRendered := ui.Render(theme.Dimmed, prefix+connector)

	name := node.Branch.Name
	style := theme.BranchHealthy
	switch {
	case node.Branch.Name == current:
		style = theme.BranchCurrent
		name = name + "  ← current"
	case node.NeedsRestack:
		style = theme.BranchNeedsRestack
		name = name + "  (needs restack)"
	}

	suffix := ""
	if node.PR != nil {
		suffix = " " + renderPRPill(*node.PR)
	} else if node.Branch.PR > 0 {
		suffix = ui.Render(theme.Dimmed, fmt.Sprintf(" #%d", node.Branch.PR))
	}

	if node.Status != nil {
		if opts.IncludeChecks {
			if badge := renderCheckBadge(node.Status.Checks); badge != "" {
				suffix += " " + badge
			}
		}
		if opts.IncludeMergeStatus && !node.Status.IsDraft {
			if badge := renderMergeBadge(node.Status.Mergeable); badge != "" {
				suffix += " " + badge
			}
		}
	}

	b.WriteString(connectorRendered + ui.Render(style, name) + suffix + "\n")

	for i, child := range node.Children {
		renderNode(b, child, childPrefix, i == len(node.Children)-1, current, opts)
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

// renderCheckBadge returns a coloured "CI" chip representing the CI
// rollup, or "" when there are no checks at all (a config-less repo
// would otherwise carry a permanent neutral badge per row). The
// label is intentionally constant across states so the column aligns
// — colour, not text, communicates pass / pending / fail.
func renderCheckBadge(rollup gh.CheckRollup) string {
	switch rollup {
	case gh.ChecksPass:
		return ui.Render(theme.BadgeCIPass, "CI")
	case gh.ChecksPending:
		return ui.Render(theme.BadgeCIPending, "CI")
	case gh.ChecksFail:
		return ui.Render(theme.BadgeCIFail, "CI")
	default:
		return ""
	}
}

// renderMergeBadge returns the per-row mergeability indicator as a
// chip distinct from the CI badge: green "ready" when GitHub says
// MERGEABLE, orange "conflict" when CONFLICTING. UNKNOWN suppresses
// the badge so the column doesn't get noisy while GitHub is still
// computing the merge state.
func renderMergeBadge(m gh.Mergeability) string {
	switch m {
	case gh.MergeMergeable:
		return ui.Render(theme.BadgeMergeReady, "ready")
	case gh.MergeConflicting:
		return ui.Render(theme.BadgeMergeConflict, "conflict")
	default:
		return ""
	}
}
