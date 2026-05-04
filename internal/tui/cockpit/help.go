package cockpit

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
)

// helpSection groups related bindings under one rendered header.
// Pulling the layout out as data — rather than hardcoding the
// rendering site-by-site — means new bindings join the help
// overlay by adding one line to buildHelpSections, not by editing
// view code.
type helpSection struct {
	Title    string
	Bindings []KeyBinding
}

// buildHelpSections is the single source of truth for what the
// help overlay shows and how it's grouped. Every binding in the
// keymap appears under exactly one section so the help can claim
// completeness — a missing binding here is a discoverability bug.
//
// Sections are ordered by user-frequency: things you press first
// (navigation, dashboard actions) live above things you press
// rarely (conflict resolver, palette select).
func buildHelpSections(k Keymap) []helpSection {
	return []helpSection{
		{
			Title: "navigation",
			Bindings: []KeyBinding{
				k.Up, k.Down, k.Top, k.Bottom,
			},
		},
		{
			Title: "global",
			Bindings: []KeyBinding{
				k.Refresh, k.PaletteOpen, k.Help, k.Back, k.Quit,
			},
		},
		{
			Title: "local actions",
			Bindings: []KeyBinding{
				k.Checkout, k.Restack, k.Modify, k.Fold,
				k.Absorb, k.Undo, k.Track, k.Untrack, k.Diff,
			},
		},
		{
			Title: "network actions",
			Bindings: []KeyBinding{
				k.Submit, k.Sync, k.Land,
			},
		},
		{
			Title: "conflict resolver",
			Bindings: []KeyBinding{
				k.ConflictContinue, k.ConflictAbort, k.ConflictEdit,
			},
		},
		{
			Title: "diff viewer",
			Bindings: []KeyBinding{
				k.DiffPageUp, k.DiffPageDown,
				k.DiffPrevCommit, k.DiffNextCommit,
			},
		},
		{
			Title: "command palette",
			Bindings: []KeyBinding{
				k.PaletteSelect,
			},
		},
	}
}

// updateHelp handles help-screen keys. Only Esc/Quit need to do
// anything — the help overlay is read-only by design. Quit goes
// through the global handler so we don't even handle it here.
func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keys.Back.Matches(msg) || m.keys.Help.Matches(msg) {
		// Press ? again to toggle off — matches the muscle memory
		// from most modern editors / TUIs.
		return m.closeHelp(), nil
	}
	return m, nil
}

// openHelp transitions into the help screen, remembering where to
// return to. Same pattern as openPalette so closeHelp doesn't
// always dump the user back on the dashboard.
func (m Model) openHelp() Model {
	m.prevScreen = m.screen
	m.screen = screenHelp
	return m
}

// closeHelp pops back to the previous screen. Defensive against a
// stale prevScreen pointing at help itself.
func (m Model) closeHelp() Model {
	target := m.prevScreen
	if target == screenHelp {
		target = screenDashboard
	}
	m.screen = target
	m.prevScreen = screenDashboard
	return m
}

// viewHelp renders the help overlay. Frame the body with
// theme.OuterBorder so it reads as a transient sheet — same visual
// language the palette uses to signal "this is over your work, not
// part of it".
func (m Model) viewHelp() string {
	sections := buildHelpSections(m.keys)

	width := m.width
	if width <= 0 {
		width = dashboardDefaultWidth
	}
	innerW := width - 6
	if innerW < 40 {
		innerW = 40
	}

	var b strings.Builder
	b.WriteString(ui.Render(theme.Header, "key bindings") + "\n\n")

	// Two columns of sections so the overlay fits in a typical
	// terminal without scrolling. Lay them out by alternating
	// section index → column so balanced sections sit next to
	// each other rather than the long ones piling up on one side.
	left, right := splitSections(sections)
	leftBlock := renderHelpSections(left, m.keys)
	rightBlock := renderHelpSections(right, m.keys)

	col := innerW / 2
	if col < 24 {
		col = 24
	}
	leftCol := lipgloss.NewStyle().Width(col).Render(leftBlock)
	rightCol := lipgloss.NewStyle().Width(innerW-col).Padding(0, 1).Render(rightBlock)
	cols := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)
	b.WriteString(cols)

	body := theme.OuterBorder.Width(innerW + 2).Render(b.String())
	hint := ui.Render(theme.Dimmed, "press ? again or esc to close")
	return lipgloss.JoinVertical(lipgloss.Left, body, "", hint)
}

// splitSections distributes sections into left / right columns.
// Even-indexed sections go left, odd-indexed go right — keeps
// related groupings (e.g. navigation + global) visually adjacent
// rather than piling all the long ones on one side.
func splitSections(sections []helpSection) ([]helpSection, []helpSection) {
	left := make([]helpSection, 0, len(sections)/2+1)
	right := make([]helpSection, 0, len(sections)/2+1)
	for i, s := range sections {
		if i%2 == 0 {
			left = append(left, s)
		} else {
			right = append(right, s)
		}
	}
	return left, right
}

// renderHelpSections renders a column of sections with the same
// "key  label" layout used elsewhere in the cockpit so the help
// overlay matches the inline footer hints visually.
func renderHelpSections(sections []helpSection, _ Keymap) string {
	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ui.Render(theme.Subtitle, sec.Title) + "\n")
		for _, bd := range sec.Bindings {
			keys := strings.Join(bd.Keys, "/")
			line := "  " +
				ui.Render(theme.KeyHint, padRight(keys, 14)) + " " +
				ui.Render(theme.KeyLabel, bd.Help)
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// padRight returns s padded with spaces up to width. Used so the
// help overlay's key column lines up vertically across rows
// regardless of binding-key length. We deliberately don't pad with
// lipgloss styles because measuring rendered width through ANSI
// escape codes is finicky and these strings are pure ASCII.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
