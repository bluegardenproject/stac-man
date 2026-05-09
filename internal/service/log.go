package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/progress"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
)

// LogOptions configures Log.
type LogOptions struct {
	// IncludePRStatus, when true, decorates nodes with cached PR
	// summaries. `sm log` does not refresh GitHub state; `sm sync`
	// and `sm status` own freshness.
	IncludePRStatus bool
	// IncludeChecks is retained for output compatibility while live
	// GitHub status moves behind `sm status`. It is only rendered when
	// status data was supplied by a caller.
	IncludeChecks bool
	// IncludeMergeStatus is retained for output compatibility while
	// live GitHub status moves behind `sm status`. Drafts and closed
	// PRs never render a glyph regardless of this flag.
	IncludeMergeStatus bool
	// Progress is retained for callers that share LogOptions while
	// live GitHub status moves behind explicit refresh commands.
	Progress progress.Reporter
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

// LogResult is the structured form of `sm log`. JSON tags drive
// `sm log --json`. The trunk is reported once at the top level and
// is intentionally NOT repeated as a branch row — every entry in
// Branches is a tracked (non-trunk) branch. Order is depth-first
// pre-order over each root in name-sorted order, so a parent always
// appears before its descendants and the same input graph always
// produces the same output.
type LogResult struct {
	Trunk    string       `json:"trunk"`
	Current  string       `json:"current,omitempty"`
	Branches []*LogBranch `json:"branches"`
}

// LogBranch is one tracked-branch row in a LogResult. Field names
// overlap with BranchView (from `sm show --json`) so a JSON consumer
// can write a single per-branch parser; the embedded *PRView is the
// same type both commands use, including its optional Checks /
// Mergeable fields. Children carries only the immediate child names
// — the full subtree is reconstructible from the flat Branches list
// via Parent links, so we don't duplicate the recursive shape here.
type LogBranch struct {
	Branch       string   `json:"branch"`
	Parent       string   `json:"parent,omitempty"`
	ParentSHA    string   `json:"parentSHA,omitempty"`
	Depth        int      `json:"depth"`
	IsCurrent    bool     `json:"isCurrent,omitempty"`
	NeedsRestack bool     `json:"needsRestack,omitempty"`
	Children     []string `json:"children,omitempty"`
	PR           *PRView  `json:"pr,omitempty"`
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

// LogData returns the structured graph `sm log` builds, ready for
// machine consumption. It runs the same gather phase as Log but
// skips the rendered-tree formatter — callers get back a
// fully-populated *LogResult they can JSON-encode, format as
// porcelain rows, or transform further. Used by `sm log --json` and
// `sm log --porcelain`.
func (s *Service) LogData(ctx context.Context, opts LogOptions) (*LogResult, error) {
	d, err := s.buildLogData(ctx, opts)
	if err != nil {
		return nil, err
	}
	return logResultFromData(d), nil
}

// logResultFromData maps the unexported logData (used by the rendered
// tree) onto the public LogResult shape. Pure mapping function so
// unit tests can exercise it without a Service.
func logResultFromData(d *logData) *LogResult {
	out := &LogResult{
		Trunk:    d.Trunk,
		Current:  d.Current,
		Branches: []*LogBranch{},
	}
	for _, root := range d.Roots {
		appendLogBranch(&out.Branches, root, 1, d.Current)
	}
	return out
}

func appendLogBranch(out *[]*LogBranch, n *logBranchNode, depth int, current string) {
	lb := &LogBranch{
		Branch:       n.Branch.Name,
		Parent:       n.Branch.Parent,
		ParentSHA:    n.Branch.ParentSHA,
		Depth:        depth,
		IsCurrent:    n.Branch.Name == current,
		NeedsRestack: n.NeedsRestack,
	}
	for _, c := range n.Children {
		lb.Children = append(lb.Children, c.Branch.Name)
	}
	switch {
	case n.PR != nil:
		lb.PR = &PRView{
			Number: n.PR.Number,
			State:  string(n.PR.State),
			URL:    n.PR.URL,
			Draft:  n.PR.IsDraft,
			Title:  n.PR.Title,
		}
		if n.Status != nil {
			lb.PR.Checks = n.Status.Checks
			lb.PR.Mergeable = n.Status.Mergeable
		}
	case n.Branch.PR > 0:
		// Recorded PR number but no fresh data (e.g. --no-pr was
		// passed). Surface the number alone so consumers still see
		// the link to the GitHub PR.
		lb.PR = &PRView{Number: n.Branch.PR}
	}
	*out = append(*out, lb)
	for _, c := range n.Children {
		appendLogBranch(out, c, depth+1, current)
	}
}

// LogPorcelainColumns is the canonical column order for
// `sm log --porcelain`. Stable across releases — adding a column
// means appending; existing scripts must keep working unchanged.
var LogPorcelainColumns = []string{
	"branch", "parent", "depth", "pr_number", "pr_state", "ci", "mergeable", "is_current", "needs_restack",
}

// FormatLogPorcelain renders a LogResult as the documented tab-
// separated porcelain format: one row per tracked branch, fields in
// LogPorcelainColumns order, empty fields written as "-". No header
// line — the column order is the contract.
func FormatLogPorcelain(r *LogResult) string {
	var b strings.Builder
	for _, lb := range r.Branches {
		fields := []string{
			lb.Branch,
			dashIfEmpty(lb.Parent),
			fmt.Sprintf("%d", lb.Depth),
			"-", "-", "-", "-",
			boolWord(lb.IsCurrent),
			boolWord(lb.NeedsRestack),
		}
		if lb.PR != nil {
			if lb.PR.Number > 0 {
				fields[3] = fmt.Sprintf("%d", lb.PR.Number)
			}
			fields[4] = porcelainPRState(lb.PR)
			fields[5] = porcelainCI(lb.PR.Checks)
			fields[6] = porcelainMerge(lb.PR.Mergeable)
		}
		b.WriteString(strings.Join(fields, "\t"))
		b.WriteByte('\n')
	}
	return b.String()
}

// porcelainPRState collapses the PRView state + Draft flag into a
// single token. Matches the labels used in the rendered tree (open /
// draft / merged / closed) so users see consistent vocabulary
// whether they're reading the synthwave tree or grepping porcelain
// output.
func porcelainPRState(pr *PRView) string {
	if pr == nil || pr.Number == 0 {
		return "-"
	}
	if pr.Draft {
		return "draft"
	}
	switch pr.State {
	case "MERGED":
		return "merged"
	case "CLOSED":
		return "closed"
	case "OPEN":
		return "open"
	case "":
		return "-"
	default:
		return strings.ToLower(pr.State)
	}
}

func porcelainCI(c gh.CheckRollup) string {
	switch c {
	case gh.ChecksPass:
		return "pass"
	case gh.ChecksPending:
		return "pending"
	case gh.ChecksFail:
		return "fail"
	default:
		return "-"
	}
}

func porcelainMerge(m gh.Mergeability) string {
	switch m {
	case gh.MergeMergeable:
		return "mergeable"
	case gh.MergeConflicting:
		return "conflicting"
	default:
		return "-"
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func boolWord(v bool) string {
	if v {
		return "true"
	}
	return "false"
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

	needsRestack := s.computeNeedsRestackMap(ctx, g)
	return s.buildLogDataWithGraph(ctx, trunk, current, g, opts, needsRestack), nil
}

func (s *Service) buildLogDataWithGraph(ctx context.Context, trunk, current string, g *stack.Graph, opts LogOptions, needsRestack map[string]bool) *logData {
	prMap := map[string]gh.PR{}
	statusMap := map[string]gh.PRStatus{}
	if opts.IncludePRStatus {
		prMap = s.cachedPRs(ctx, g.Branches())
	}
	return &logData{
		Trunk:   trunk,
		Current: current,
		Roots:   buildLogTree(g, prMap, statusMap, needsRestack),
	}
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
