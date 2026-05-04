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

// diffDefaultPaneHeight is used before the first WindowSizeMsg
// arrives so the viewport math has stable values to render against.
const diffDefaultPaneHeight = 24

// diffSidebarShare controls how much horizontal width the commit
// list takes; the remainder belongs to the diff content. A 3/10
// split keeps SHA + subject readable without crowding the diff
// pane, where horizontal lines tend to need every column.
const diffSidebarShare = 3

// diffMsg is delivered when an asynchronous CommitDiff load
// completes. Mirrors detailMsg's design — the sha key lets Update
// route the result to the right cache entry even when the user has
// since stepped to another commit.
type diffMsg struct {
	sha     string
	content string
	err     error
}

// loadDiffCmd fetches one commit's unified diff via Service.CommitDiff.
// sha is captured in the closure so the diffMsg always carries the
// right cache key.
func loadDiffCmd(m Model, sha string) tea.Cmd {
	return func() tea.Msg {
		out, err := m.svc.CommitDiff(m.ctx, sha)
		return diffMsg{sha: sha, content: out, err: err}
	}
}

// updateDiff handles diff-screen keys. Layout split:
//   - cursor/page/Top/Bottom = scroll the diff content;
//   - DiffNextCommit / DiffPrevCommit = step through the commit list,
//     resetting the scroll offset so the next commit starts at top;
//   - Back = pop back to the dashboard.
//
// Routing entry from the dashboard guarantees diffCommits is
// non-empty when this handler runs; defensive checks here exist so
// a stray Update during an in-flight branch switch can't crash.
func (m Model) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.diffCommits) == 0 {
		if m.keys.Back.Matches(msg) {
			m = m.exitDiff()
		}
		return m, nil
	}

	height := diffViewportHeight(m)

	switch {
	case m.keys.Back.Matches(msg):
		m = m.exitDiff()
		return m, nil

	case m.keys.DiffNextCommit.Matches(msg):
		if m.diffCommitCursor < len(m.diffCommits)-1 {
			m.diffCommitCursor++
			m.diffScroll = 0
			return m, m.diffCmdForSelection()
		}
		return m, nil
	case m.keys.DiffPrevCommit.Matches(msg):
		if m.diffCommitCursor > 0 {
			m.diffCommitCursor--
			m.diffScroll = 0
			return m, m.diffCmdForSelection()
		}
		return m, nil

	case m.keys.Up.Matches(msg):
		if m.diffScroll > 0 {
			m.diffScroll--
		}
		return m, nil
	case m.keys.Down.Matches(msg):
		m.diffScroll = clampDiffScroll(m.diffScroll+1, m, height)
		return m, nil
	case m.keys.DiffPageUp.Matches(msg):
		m.diffScroll -= height
		if m.diffScroll < 0 {
			m.diffScroll = 0
		}
		return m, nil
	case m.keys.DiffPageDown.Matches(msg):
		m.diffScroll = clampDiffScroll(m.diffScroll+height, m, height)
		return m, nil
	case m.keys.Top.Matches(msg):
		m.diffScroll = 0
		return m, nil
	case m.keys.Bottom.Matches(msg):
		m.diffScroll = clampDiffScroll(maxDiffScroll(m, height), m, height)
		return m, nil
	}
	return m, nil
}

// diffViewportHeight is the number of diff-content lines that fit
// in the viewport. We subtract a fixed budget for the header,
// commit-row line, status (when present), and footer so the
// viewport never overdraws the surrounding chrome. A minimum keeps
// pathologically small terminals from yielding zero.
func diffViewportHeight(m Model) int {
	if m.height <= 0 {
		return diffDefaultPaneHeight
	}
	const chromeBudget = 8
	h := m.height - chromeBudget
	if h < 4 {
		return 4
	}
	return h
}

// maxDiffScroll returns the largest scroll offset that still shows
// at least one line of content. Cached diff lookups feed this so a
// not-yet-loaded commit treats max as 0 (no scroll possible until
// content arrives).
func maxDiffScroll(m Model, height int) int {
	sha := selectedCommitSHA(m)
	content, ok := m.diffCache[sha]
	if !ok {
		return 0
	}
	lines := strings.Count(content, "\n")
	max := lines - height + 1
	if max < 0 {
		return 0
	}
	return max
}

// clampDiffScroll keeps the scroll offset in [0, max]. Pulled out
// so the page/down/end handlers don't repeat the bounds dance.
func clampDiffScroll(want int, m Model, height int) int {
	max := maxDiffScroll(m, height)
	if want < 0 {
		return 0
	}
	if want > max {
		return max
	}
	return want
}

// diffCmdForSelection returns a load command for the commit under
// the cursor when its diff isn't already cached. Mirrors
// detailCmdForSelection's caching contract so cursor stepping
// doesn't re-fetch the same diff on every move.
func (m Model) diffCmdForSelection() tea.Cmd {
	sha := selectedCommitSHA(m)
	if sha == "" {
		return nil
	}
	if _, ok := m.diffCache[sha]; ok {
		return nil
	}
	if _, ok := m.diffErrs[sha]; ok {
		return nil
	}
	return loadDiffCmd(m, sha)
}

// selectedCommitSHA returns the SHA of the commit under the diff
// cursor, or "" when the commit list is empty / cursor out of
// bounds. Using a helper keeps callers from re-implementing the
// bounds dance.
func selectedCommitSHA(m Model) string {
	if len(m.diffCommits) == 0 {
		return ""
	}
	if m.diffCommitCursor < 0 || m.diffCommitCursor >= len(m.diffCommits) {
		return ""
	}
	return m.diffCommits[m.diffCommitCursor].SHA
}

// exitDiff resets the diff-screen state and routes back to the
// dashboard. Done in a method so multiple Back paths (no commits +
// real Back) share the cleanup.
func (m Model) exitDiff() Model {
	m.screen = screenDashboard
	m.diffBranch = ""
	m.diffCommits = nil
	m.diffCommitCursor = 0
	m.diffScroll = 0
	// The cache is intentionally NOT cleared — diffs are immutable
	// per SHA, so re-entering the viewer for the same branch
	// should reuse them. A manual refresh on the dashboard wipes
	// the cache when the user wants fresh reads.
	return m
}

// enterDiff prepares the model for a transition from the dashboard
// into the diff viewer. Pulled out of the dashboard's dispatch so
// the entry semantics live next to the rest of the diff state.
//
// commits is the BranchView.Commits slice taken from the cached
// detail; pulling it once at entry rather than re-reading from the
// cache on every render means the viewer is unaffected by an
// async refresh that overwrites m.details mid-session.
func (m Model) enterDiff(branch string, commits []service.CommitView) (Model, tea.Cmd) {
	m.screen = screenDiff
	m.diffBranch = branch
	m.diffCommits = commits
	m.diffCommitCursor = 0
	m.diffScroll = 0
	if m.diffCache == nil {
		m.diffCache = map[string]string{}
	}
	if m.diffErrs == nil {
		m.diffErrs = map[string]error{}
	}
	return m, m.diffCmdForSelection()
}

// viewDiff renders the diff screen. Three sections, in order:
//
//   - Header naming the branch
//   - Two-pane body: commit list (left) + diff content (right)
//   - Footer with the per-screen key hints
//
// An empty-commits state (untracked branch, fresh-from-trunk
// branch) renders a friendly placeholder rather than an empty
// pane.
func (m Model) viewDiff() string {
	var b strings.Builder
	b.WriteString(ui.Banner("stac-man — diff") + "\n\n")
	b.WriteString(ui.Render(theme.Header, m.diffBranch) + "\n\n")

	if len(m.diffCommits) == 0 {
		b.WriteString(ui.Render(theme.Dimmed, "(no commits unique to this branch — press esc to return)") + "\n\n")
		b.WriteString(viewDiffFooter(m))
		return b.String()
	}

	b.WriteString(viewDiffPanes(m))

	if status := viewActionStatus(m.lastAction); status != "" {
		b.WriteString("\n" + status + "\n")
	}

	b.WriteString("\n")
	b.WriteString(viewDiffFooter(m))
	return b.String()
}

// viewDiffPanes lays the commit list and diff viewport side by
// side. Width math mirrors viewDashboardPanes so the cockpit feels
// visually consistent across screens.
func viewDiffPanes(m Model) string {
	width := m.width
	if width <= 0 {
		width = dashboardDefaultWidth
	}
	sideW := width * diffSidebarShare / 10
	if sideW < 24 {
		sideW = 24
	}
	bodyW := width - sideW
	if bodyW < 24 {
		bodyW = 24
	}

	height := diffViewportHeight(m)
	side := lipgloss.NewStyle().
		Width(sideW).
		Render(viewDiffCommits(m))
	body := lipgloss.NewStyle().
		Width(bodyW).
		Padding(0, 1).
		Render(viewDiffBody(m, height))

	return lipgloss.JoinHorizontal(lipgloss.Top, side, body)
}

// viewDiffCommits renders the cursor-driven commit list. Each row
// is short-SHA + subject; the cursor row gets the same caret +
// bold treatment used everywhere else in the cockpit so selection
// is unambiguous.
func viewDiffCommits(m Model) string {
	var b strings.Builder
	b.WriteString(ui.Render(theme.KeyLabel, fmt.Sprintf("commits (%d):", len(m.diffCommits))) + "\n")
	for i, c := range m.diffCommits {
		caret := "  "
		if i == m.diffCommitCursor {
			caret = ui.Render(theme.Accent, "▸ ")
		}
		row := caret +
			ui.Render(theme.Dimmed, shortSHA(c.SHA)) + " " +
			c.Subject
		if i == m.diffCommitCursor {
			row = ui.Render(theme.Bold, row)
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}

// viewDiffBody renders the diff content for the selected commit
// inside the viewport window [diffScroll, diffScroll+height).
// Three states converge here: error cached, content cached,
// neither yet (loading placeholder).
func viewDiffBody(m Model, height int) string {
	sha := selectedCommitSHA(m)
	if sha == "" {
		return ui.Render(theme.Dimmed, "no commit selected")
	}
	if err, ok := m.diffErrs[sha]; ok {
		return ui.Render(theme.Fail, fmt.Sprintf("git show %s failed:\n", shortSHA(sha))) + err.Error()
	}
	content, ok := m.diffCache[sha]
	if !ok {
		return ui.Render(theme.Dimmed, "loading diff…")
	}

	lines := strings.Split(content, "\n")
	end := m.diffScroll + height
	if end > len(lines) {
		end = len(lines)
	}
	start := m.diffScroll
	if start > len(lines) {
		start = len(lines)
	}
	visible := lines[start:end]

	var b strings.Builder
	for _, line := range visible {
		b.WriteString(colourDiffLine(line))
		b.WriteByte('\n')
	}
	if total := len(lines); total > height {
		b.WriteString(ui.Render(theme.Dimmed, fmt.Sprintf("\n[%d-%d of %d]", start+1, end, total)))
	}
	return b.String()
}

// colourDiffLine paints unified-diff lines with the conventional
// red/green/cyan mapping so the viewer reads at a glance. The
// hunk-header prefix `@@` and file headers share the cyan accent
// to match git's own --color output.
func colourDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "@@"):
		return ui.Render(theme.Info, line)
	case strings.HasPrefix(line, "diff --git"),
		strings.HasPrefix(line, "index "),
		strings.HasPrefix(line, "--- "),
		strings.HasPrefix(line, "+++ "):
		return ui.Render(theme.Header, line)
	case strings.HasPrefix(line, "+"):
		return ui.Render(theme.OK, line)
	case strings.HasPrefix(line, "-"):
		return ui.Render(theme.Fail, line)
	default:
		return line
	}
}

// viewDiffFooter renders the diff-screen key hints. Reads from
// m.keys so binding renames anywhere in keymap.go propagate here
// automatically.
func viewDiffFooter(m Model) string {
	hint := func(b KeyBinding) string {
		key := strings.Join(b.Keys, "/")
		return ui.Render(theme.KeyHint, key) + " " + ui.Render(theme.KeyLabel, b.Help)
	}
	scroll := []string{
		hint(m.keys.Up),
		hint(m.keys.Down),
		hint(m.keys.DiffPageUp),
		hint(m.keys.DiffPageDown),
		hint(m.keys.Top),
		hint(m.keys.Bottom),
	}
	commit := []string{
		hint(m.keys.DiffPrevCommit),
		hint(m.keys.DiffNextCommit),
		hint(m.keys.Back),
		hint(m.keys.Quit),
	}
	return strings.Join(scroll, "  •  ") + "\n" + strings.Join(commit, "  •  ")
}
