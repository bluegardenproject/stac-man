package cockpit

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
)

// dashboardDefaultWidth is used before the first WindowSizeMsg
// arrives so the View has a stable layout to render against.
const dashboardDefaultWidth = 80

// dashboardTreeShare controls how much of the available width the
// tree pane gets; the remainder belongs to the detail pane. 4/10
// keeps long branch names mostly visible without crowding the
// detail pane.
const dashboardTreeShare = 4

// updateDashboard handles dashboard-only key messages: cursor
// movement and the local-action bindings. Global keys (Quit,
// Refresh, Help) are handled by Update before this is reached, so
// this stays focused on what's relative to the selected row or the
// current branch.
func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.snapshot == nil || len(m.snapshot.CheckoutItems) == 0 {
		return m, nil
	}

	// Screen-changing keys (e.g. Diff opens the diff viewer) are
	// dispatched first so they pre-empt any action that happens
	// to share the binding's String().
	if next, cmd, ok := m.dispatchDashboardKey(msg); ok {
		return next, cmd
	}

	// Action dispatch. Cursor-targeted actions read the row under
	// the cursor; HEAD-targeted ones (modify/fold/absorb/undo)
	// ignore the cursor entirely. Trunk row is rejected for the
	// mutating cursor-targeted bindings so an accidental press on
	// `main` doesn't produce a confusing service-layer error in the
	// status line.
	if cmd := m.dispatchActionKey(msg); cmd != nil {
		return m, cmd
	}

	n := len(m.snapshot.CheckoutItems)
	prev := m.cursor
	switch {
	case m.keys.Up.Matches(msg):
		if m.cursor > 0 {
			m.cursor--
		}
	case m.keys.Down.Matches(msg):
		if m.cursor < n-1 {
			m.cursor++
		}
	case m.keys.Top.Matches(msg):
		m.cursor = 0
	case m.keys.Bottom.Matches(msg):
		m.cursor = n - 1
	}
	if m.cursor != prev {
		return m, m.detailCmdForSelection()
	}
	return m, nil
}

// dispatchActionKey returns the tea.Cmd for the matched action
// binding, or nil when msg doesn't map to any action. Pulled out
// of updateDashboard so the cursor-movement switch stays readable
// and so tests can target action dispatch without driving the
// arrow-key paths.
//
// Cursor-targeted actions silently no-op when:
//   - there is no selected row (defensive — the empty-stack early
//     return in updateDashboard already covers the common case);
//   - the selected row is the trunk for actions that can't
//     reasonably target it (restack, track, untrack).
//
// HEAD-targeted actions don't gate on cursor state because the
// service layer enforces "must not be on trunk" with a clear error
// that the status line will surface verbatim.
func (m Model) dispatchActionKey(msg tea.KeyMsg) tea.Cmd {
	cur, hasCur := m.selectedItem()

	switch {
	case m.keys.Checkout.Matches(msg):
		if !hasCur {
			return nil
		}
		return m.checkoutCmd(cur.Branch)
	case m.keys.Restack.Matches(msg):
		if !hasCur || cur.IsTrunk {
			return nil
		}
		return m.restackCmd(cur.Branch)
	case m.keys.Track.Matches(msg):
		if !hasCur || cur.IsTrunk {
			return nil
		}
		return m.trackCmd(cur.Branch)
	case m.keys.Untrack.Matches(msg):
		if !hasCur || cur.IsTrunk {
			return nil
		}
		return m.untrackCmd(cur.Branch)
	case m.keys.Modify.Matches(msg):
		return m.modifyCmd()
	case m.keys.Fold.Matches(msg):
		return m.foldCmd()
	case m.keys.Absorb.Matches(msg):
		return m.absorbCmd()
	case m.keys.Undo.Matches(msg):
		return m.undoCmd()

	// Network actions. These shell out via tea.ExecProcess and
	// resolve to a networkFinishedMsg the model handles
	// separately from in-process actionResult outcomes.
	case m.keys.Submit.Matches(msg):
		return m.submitCmd()
	case m.keys.Sync.Matches(msg):
		return m.syncCmd()
	case m.keys.Land.Matches(msg):
		return m.landCmd()
	}
	return nil
}

// dispatchDashboardKey is a separate switch from dispatchActionKey
// for keys that change the *screen* rather than firing a service
// call. Pulled out so the action dispatcher stays focused on
// actionResult-producing commands; this one returns a model
// transformation directly.
//
// Returns ok=false when the key didn't match any dashboard-screen
// gesture so the caller can keep walking the cursor / refresh
// fall-throughs.
func (m Model) dispatchDashboardKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if !m.keys.Diff.Matches(msg) {
		return m, nil, false
	}
	cur, ok := m.selectedItem()
	if !ok || cur.IsTrunk {
		// Trunk has no per-branch commits to diff. Silent no-op
		// rather than a noisy "trunk has no diff" status — the
		// user will press d on a real branch next.
		return m, nil, true
	}
	view, cached := m.details[cur.Branch]
	if !cached || view == nil {
		// We need the BranchView's commit list to populate the
		// viewer. If detail isn't cached yet (rare — Show fires
		// on cursor moves), surface a status hint and stay on
		// the dashboard rather than entering an empty viewer.
		m.lastAction = &actionResult{
			Verb:   m.keys.Diff.Help,
			Branch: cur.Branch,
			Err:    fmt.Errorf("branch detail not loaded yet — try again"),
		}
		return m, nil, true
	}
	next, cmd := m.enterDiff(cur.Branch, view.Commits)
	return next, cmd, true
}

// selectedItem returns the CheckoutItem under the cursor and a
// presence flag. Saves callers from re-implementing the bounds
// dance and lets dispatchActionKey gate on item flags (IsTrunk,
// IsCurrent) without index arithmetic.
func (m Model) selectedItem() (service.CheckoutItem, bool) {
	if m.snapshot == nil {
		return service.CheckoutItem{}, false
	}
	items := m.snapshot.CheckoutItems
	if m.cursor < 0 || m.cursor >= len(items) {
		return service.CheckoutItem{}, false
	}
	return items[m.cursor], true
}

// viewDashboard renders the dashboard at its current state. Three
// rendering paths converge here so the screen never goes blank
// between paints: load error, pre-snapshot loading, and the real
// two-pane view.
func (m Model) viewDashboard() string {
	var b strings.Builder
	b.WriteString(ui.Banner("stac-man") + "\n\n")

	switch {
	case m.loadErr != nil:
		b.WriteString(ui.Render(theme.Fail, "load failed: ") + m.loadErr.Error() + "\n")
	case m.snapshot == nil:
		b.WriteString(ui.Render(theme.Dimmed, "loading…") + "\n")
	default:
		if m.snapshot.Paused != nil {
			b.WriteString(viewPausedBanner(m.snapshot.Paused) + "\n\n")
		}
		b.WriteString(viewDashboardPanes(m))
	}

	if status := viewActionStatus(m.lastAction); status != "" {
		b.WriteString("\n")
		b.WriteString(status)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(viewDashboardFooter(m))
	return b.String()
}

// viewActionStatus renders the last action's outcome below the
// dashboard panes. The text comes from statusLine() (single source
// of formatting); the colour band is decided here so the styling
// stays adjacent to the other view code. Returns "" when there's
// nothing to show so the caller can skip the spacing line.
func viewActionStatus(res *actionResult) string {
	text := statusLine(res)
	if text == "" {
		return ""
	}
	switch {
	case res.Paused != nil:
		return ui.Render(theme.Warn, text)
	case res.Err != nil:
		return ui.Render(theme.Fail, text)
	default:
		return ui.Render(theme.OK, text)
	}
}

// viewPausedBanner renders the persistent warning shown above the
// dashboard whenever a stac-man op is paused. Surfacing it on every
// refresh prevents the user from forgetting they have a stuck
// rebase that needs `sm continue` (or, soon, the in-TUI resolver).
func viewPausedBanner(p *service.PausedSnapshot) string {
	branch := p.Branch
	if branch == "" {
		branch = "(unknown)"
	}
	return ui.Render(theme.ErrorToast, fmt.Sprintf(" paused on %s — %d conflict(s)", branch, len(p.ConflictPaths)))
}

// viewDashboardPanes composes the tree pane (left) with the detail
// pane (right) using lipgloss.JoinHorizontal. Widths default to a
// sane 80-column layout before the first WindowSizeMsg lands.
func viewDashboardPanes(m Model) string {
	width := m.width
	if width <= 0 {
		width = dashboardDefaultWidth
	}
	treeW := width * dashboardTreeShare / 10
	if treeW < 24 {
		treeW = 24
	}
	detailW := width - treeW
	if detailW < 24 {
		detailW = 24
	}

	tree := lipgloss.NewStyle().
		Width(treeW).
		Render(viewTree(m, treeW))
	detail := lipgloss.NewStyle().
		Width(detailW).
		Padding(0, 1).
		Render(viewDetail(m, detailW))

	return lipgloss.JoinHorizontal(lipgloss.Top, tree, detail)
}

// viewTree renders the snapshot's CheckoutItems as a navigable tree
// with the cursor on m.cursor. The connector logic mirrors
// internal/tui/picker.go so the cockpit's tree feels identical to
// the standalone picker.
func viewTree(m Model, _ int) string {
	if m.snapshot == nil {
		return ""
	}
	items := m.snapshot.CheckoutItems
	if len(items) == 0 {
		return ui.Render(theme.Dimmed, "(no tracked branches)\n")
	}
	var b strings.Builder
	for i, it := range items {
		b.WriteString(renderTreeRow(it, i == m.cursor))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderTreeRow draws one row of the tree pane. Selection is shown
// by a cyan caret + a bold pass over the row so it pops at any
// terminal contrast.
func renderTreeRow(it service.CheckoutItem, selected bool) string {
	caret := "  "
	if selected {
		caret = ui.Render(theme.Accent, "▸ ")
	}

	var prefix strings.Builder
	for _, ancestorIsLast := range it.AncestorIsLast {
		if ancestorIsLast {
			prefix.WriteString("   ")
		} else {
			prefix.WriteString("│  ")
		}
	}
	if !it.IsTrunk {
		if it.IsLastChild {
			prefix.WriteString("└─ ")
		} else {
			prefix.WriteString("├─ ")
		}
	}

	style := branchStyleFor(it)
	name := it.Branch
	suffix := ""
	switch {
	case it.IsCurrent:
		suffix = "  ← current"
	case it.NeedsRestack:
		suffix = "  (needs restack)"
	}

	row := caret + ui.Render(theme.Dimmed, prefix.String()) + ui.Render(style, name+suffix)
	if it.PR > 0 {
		row += ui.Render(theme.Dimmed, fmt.Sprintf("  #%d", it.PR))
	}
	if selected {
		row = lipgloss.NewStyle().Bold(true).Render(row)
	}
	return row
}

func branchStyleFor(it service.CheckoutItem) lipgloss.Style {
	switch {
	case it.IsTrunk:
		return theme.BranchTrunk
	case it.IsCurrent:
		return theme.BranchCurrent
	case it.NeedsRestack:
		return theme.BranchNeedsRestack
	default:
		return theme.BranchHealthy
	}
}

// viewDetail renders the detail pane for the currently-cursored
// branch. Three paths converge:
//   - error cached → red banner
//   - view cached → full BranchView render
//   - neither yet → "loading…" placeholder
func viewDetail(m Model, _ int) string {
	branch := m.selectedBranch()
	if branch == "" {
		return ui.Render(theme.Dimmed, "no branch selected")
	}
	if err, ok := m.detailErrs[branch]; ok {
		return ui.Render(theme.Fail, fmt.Sprintf("show %s failed:\n", branch)) + err.Error()
	}
	view, ok := m.details[branch]
	if !ok {
		return ui.Render(theme.Header, branch) + "\n\n" + ui.Render(theme.Dimmed, "loading detail…")
	}
	return renderBranchDetail(view)
}

// renderBranchDetail formats a BranchView into the detail pane's
// vertical layout. Each section is short and labelled so the user
// can scan it without parsing prose: header, parent + restack
// state, ahead/behind counts, PR pill, child list, commit list.
func renderBranchDetail(v *service.BranchView) string {
	var b strings.Builder

	header := v.Branch
	headerStyle := theme.BranchHealthy
	switch {
	case v.Branch == v.Trunk:
		headerStyle = theme.BranchTrunk
	case v.NeedsRestack:
		headerStyle = theme.BranchNeedsRestack
	}
	b.WriteString(ui.Render(headerStyle, header))
	if v.NeedsRestack {
		b.WriteString("  " + ui.Render(theme.Warn, "(needs restack)"))
	}
	b.WriteByte('\n')

	if v.Parent != "" {
		b.WriteString(ui.Render(theme.KeyLabel, "parent: ") + v.Parent + "\n")
	} else if v.Branch == v.Trunk {
		b.WriteString(ui.Render(theme.Dimmed, "trunk branch (no parent)") + "\n")
	}

	if v.Parent != "" {
		b.WriteString(ui.Render(theme.KeyLabel, "vs parent: ") +
			fmt.Sprintf("%d ahead, %d behind\n", v.AheadParent, v.BehindParent))
	}
	if v.Trunk != "" && v.Branch != v.Trunk {
		b.WriteString(ui.Render(theme.KeyLabel, "vs trunk:  ") +
			fmt.Sprintf("%d ahead, %d behind\n", v.AheadTrunk, v.BehindTrunk))
	}

	if v.PR != nil {
		b.WriteByte('\n')
		b.WriteString(renderPRLine(v.PR))
		b.WriteByte('\n')
	}

	if len(v.Children) > 0 {
		b.WriteByte('\n')
		b.WriteString(ui.Render(theme.KeyLabel, "children:") + "\n")
		for _, c := range v.Children {
			b.WriteString("  " + ui.Render(theme.BranchHealthy, c) + "\n")
		}
	}

	if len(v.Commits) > 0 {
		b.WriteByte('\n')
		b.WriteString(ui.Render(theme.KeyLabel, fmt.Sprintf("commits (%d):", len(v.Commits))) + "\n")
		for _, c := range v.Commits {
			b.WriteString("  " +
				ui.Render(theme.Dimmed, shortSHA(c.SHA)) + " " +
				c.Subject + "\n")
		}
	}

	return b.String()
}

// renderPRLine pulls the PR pill style from the existing log
// renderer so a PR shown in the tree matches the one in the detail
// pane.
func renderPRLine(pr *service.PRView) string {
	label := fmt.Sprintf("#%d", pr.Number)
	style := theme.PROpen
	stateLabel := strings.ToLower(pr.State)
	switch {
	case pr.Draft:
		style = theme.PRDraft
		stateLabel = "draft"
	case pr.State == "MERGED":
		style = theme.PRMerged
	case pr.State == "CLOSED":
		style = theme.PRClosed
	}
	pill := ui.Render(style, label+" "+stateLabel)
	out := pill
	if pr.Title != "" {
		out += "  " + pr.Title
	}
	if pr.URL != "" {
		out += "\n" + ui.Render(theme.Dimmed, pr.URL)
	}
	return out
}

func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

// viewDashboardFooter renders the persistent key-hint strip. Reads
// from m.keys so a binding rename in keymap.go propagates here
// automatically — no parallel string to keep in sync.
//
// The strip is two lines so a 80-column terminal doesn't truncate
// the action hints: navigation + meta on the first row, action
// bindings on the second. When the help overlay (todo 11) lands,
// this can collapse back to a single Help hint, but until then the
// inline cheatsheet keeps the cockpit discoverable.
func viewDashboardFooter(m Model) string {
	hint := func(b KeyBinding) string {
		key := strings.Join(b.Keys, "/")
		return ui.Render(theme.KeyHint, key) + " " + ui.Render(theme.KeyLabel, b.Help)
	}
	nav := []string{
		hint(m.keys.Up),
		hint(m.keys.Down),
		hint(m.keys.Refresh),
		hint(m.keys.PaletteOpen),
		hint(m.keys.Help),
		hint(m.keys.Quit),
	}
	actions := []string{
		hint(m.keys.Checkout),
		hint(m.keys.Restack),
		hint(m.keys.Modify),
		hint(m.keys.Fold),
		hint(m.keys.Absorb),
		hint(m.keys.Undo),
		hint(m.keys.Track),
		hint(m.keys.Untrack),
		hint(m.keys.Diff),
	}
	network := []string{
		hint(m.keys.Submit),
		hint(m.keys.Sync),
		hint(m.keys.Land),
	}
	return strings.Join(nav, "  •  ") + "\n" +
		strings.Join(actions, "  •  ") + "\n" +
		strings.Join(network, "  •  ")
}
