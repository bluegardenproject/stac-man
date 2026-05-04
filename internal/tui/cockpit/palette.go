package cockpit

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
)

// paletteAction is one row in the command palette. The Run closure
// captures whatever operand it needs at construction time
// (selected branch for cursor-targeted actions, nothing for
// HEAD-targeted ones), so invocation is just `item.Run(m)` — no
// duplicated dispatch logic between palette and direct keypress.
//
// Label is what the user types against and what the row renders;
// Keys is the binding strings used for the right-aligned shortcut
// hint so the palette doubles as a discoverability cheat sheet.
type paletteAction struct {
	Label string
	Keys  []string
	Run   func(Model) (Model, tea.Cmd)
}

// buildPaletteItems is the single source of truth for what the
// palette can dispatch. New cockpit actions add an entry here;
// keeping this list co-located with the dispatch shape stops the
// palette from drifting away from the per-screen handlers.
//
// Cursor-targeted actions are filtered against the dashboard's
// current selection (skipped on trunk for mutating ones, hidden
// when there's no selection). HEAD-targeted ones are
// unconditional. Diff entry is added only when the cached detail
// is available — same gate as the direct `d` keypress so palette
// and keypress behave identically.
func buildPaletteItems(m Model) []paletteAction {
	out := []paletteAction{}

	cur, hasCur := m.selectedItem()
	branch := ""
	if hasCur {
		branch = cur.Branch
	}

	// Refresh — global, always available.
	out = append(out, paletteAction{
		Label: m.keys.Refresh.Help,
		Keys:  m.keys.Refresh.Keys,
		Run: func(m Model) (Model, tea.Cmd) {
			m.details = map[string]*service.BranchView{}
			m.detailErrs = map[string]error{}
			m.lastAction = nil
			return m, loadSnapshotCmd(m.ctx, m.svc)
		},
	})

	// Cursor-targeted actions. Branch is captured by value so a
	// later cursor move doesn't change which branch the palette
	// invocation targets.
	if hasCur {
		b := branch
		out = append(out, paletteAction{
			Label: labelFor(m.keys.Checkout.Help, b),
			Keys:  m.keys.Checkout.Keys,
			Run: func(m Model) (Model, tea.Cmd) {
				return m, m.checkoutCmd(b)
			},
		})
		// Diff entry — gated on detail cache the same way the
		// direct keypress is, so palette doesn't open an empty
		// viewer.
		if view, cached := m.details[b]; cached && view != nil {
			commits := view.Commits
			out = append(out, paletteAction{
				Label: labelFor(m.keys.Diff.Help, b),
				Keys:  m.keys.Diff.Keys,
				Run: func(m Model) (Model, tea.Cmd) {
					return m.enterDiff(b, commits)
				},
			})
		}
	}
	if hasCur && !cur.IsTrunk {
		b := branch
		out = append(out,
			paletteAction{
				Label: labelFor(m.keys.Restack.Help, b),
				Keys:  m.keys.Restack.Keys,
				Run: func(m Model) (Model, tea.Cmd) {
					return m, m.restackCmd(b)
				},
			},
			paletteAction{
				Label: labelFor(m.keys.Track.Help, b),
				Keys:  m.keys.Track.Keys,
				Run: func(m Model) (Model, tea.Cmd) {
					return m, m.trackCmd(b)
				},
			},
			paletteAction{
				Label: labelFor(m.keys.Untrack.Help, b),
				Keys:  m.keys.Untrack.Keys,
				Run: func(m Model) (Model, tea.Cmd) {
					return m, m.untrackCmd(b)
				},
			},
		)
	}

	// HEAD-targeted actions — always available; service layer
	// rejects on-trunk where appropriate and the rejection lands
	// in the status line.
	out = append(out,
		paletteAction{
			Label: m.keys.Modify.Help,
			Keys:  m.keys.Modify.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.modifyCmd() },
		},
		paletteAction{
			Label: m.keys.Fold.Help,
			Keys:  m.keys.Fold.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.foldCmd() },
		},
		paletteAction{
			Label: m.keys.Absorb.Help,
			Keys:  m.keys.Absorb.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.absorbCmd() },
		},
		paletteAction{
			Label: m.keys.Undo.Help,
			Keys:  m.keys.Undo.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.undoCmd() },
		},
	)

	// Network shell-outs.
	out = append(out,
		paletteAction{
			Label: m.keys.Submit.Help,
			Keys:  m.keys.Submit.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.submitCmd() },
		},
		paletteAction{
			Label: m.keys.Sync.Help,
			Keys:  m.keys.Sync.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.syncCmd() },
		},
		paletteAction{
			Label: m.keys.Land.Help,
			Keys:  m.keys.Land.Keys,
			Run:   func(m Model) (Model, tea.Cmd) { return m, m.landCmd() },
		},
	)

	return out
}

// labelFor formats a cursor-targeted action's label as
// "<verb> <branch>" so the palette query "restack feat" matches
// "restack feat-a" against branch context, not just the verb.
func labelFor(verb, branch string) string {
	if branch == "" {
		return verb
	}
	return verb + " " + branch
}

// filterPaletteItems narrows the action list down to entries
// whose label contains every whitespace-separated token in the
// query (case-insensitive). Splitting on whitespace lets users
// type "restack feat" to match "restack feat-a" without caring
// about token order. An empty query passes everything through.
func filterPaletteItems(items []paletteAction, query string) []paletteAction {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	tokens := strings.Fields(q)
	out := items[:0:0]
	for _, it := range items {
		label := strings.ToLower(it.Label)
		if matchesAllTokens(label, tokens) {
			out = append(out, it)
		}
	}
	return out
}

// matchesAllTokens is the AND-of-substring matcher backing the
// palette filter. Pulled out so tests can target it directly.
func matchesAllTokens(haystack string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(haystack, t) {
			return false
		}
	}
	return true
}

// updatePalette routes palette-screen keys. Navigation here is
// keyed off tea.KeyType (not the keymap's Up/Down/Top/Bottom)
// because those bindings include vim keys — `k`, `j`, `g`, `G`
// — that the palette must treat as query characters, otherwise
// typing "get" would scroll the cursor instead of filtering.
//
// The contract:
//   - Esc closes (matched via the keymap so synonyms work).
//   - Enter invokes the cursor item (matched via the keymap).
//   - Arrow keys move the cursor; Home/End jump to bounds.
//   - Backspace deletes one rune from the query.
//   - Anything else printable (runes / space) appends to the
//     query and resets the cursor to 0 so the new top match is
//     highlighted.
func (m Model) updatePalette(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := filterPaletteItems(buildPaletteItems(m), m.paletteQuery)

	switch {
	case m.keys.Back.Matches(msg):
		return m.closePalette(), nil
	case m.keys.PaletteSelect.Matches(msg):
		if len(items) == 0 {
			return m, nil
		}
		idx := m.paletteCursor
		if idx < 0 || idx >= len(items) {
			idx = 0
		}
		next := m.closePalette()
		return items[idx].Run(next)
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.paletteCursor > 0 {
			m.paletteCursor--
		}
		return m, nil
	case tea.KeyDown:
		if m.paletteCursor < len(items)-1 {
			m.paletteCursor++
		}
		return m, nil
	case tea.KeyHome:
		m.paletteCursor = 0
		return m, nil
	case tea.KeyEnd:
		if n := len(items); n > 0 {
			m.paletteCursor = n - 1
		}
		return m, nil
	case tea.KeyBackspace:
		if n := len(m.paletteQuery); n > 0 {
			// One-rune-at-a-time backspace; the query is ASCII
			// in practice (action labels are English) so byte
			// indexing is safe enough.
			m.paletteQuery = m.paletteQuery[:n-1]
			m.paletteCursor = 0
		}
		return m, nil
	case tea.KeyRunes:
		m.paletteQuery += string(msg.Runes)
		m.paletteCursor = 0
		return m, nil
	case tea.KeySpace:
		m.paletteQuery += " "
		m.paletteCursor = 0
		return m, nil
	}
	return m, nil
}

// openPalette transitions into the palette screen, remembering the
// previous screen so closePalette can return there. Resets the
// query and cursor so re-opening always starts on a clean slate.
func (m Model) openPalette() Model {
	m.prevScreen = m.screen
	m.screen = screenPalette
	m.paletteQuery = ""
	m.paletteCursor = 0
	return m
}

// closePalette pops back to the previous screen and clears the
// transient palette state so a stale query doesn't leak across
// re-opens.
func (m Model) closePalette() Model {
	target := m.prevScreen
	if target == screenPalette {
		// Defensive: opening the palette over itself shouldn't
		// be possible given Update's guard, but if it happens
		// we land on the dashboard rather than recursing.
		target = screenDashboard
	}
	m.screen = target
	m.prevScreen = screenDashboard
	m.paletteQuery = ""
	m.paletteCursor = 0
	return m
}

// viewPalette renders the palette's framed input + filtered list.
// We frame it with theme.Panel so it visually lifts off the screen
// and reads as a transient overlay even though, internally, it's a
// full screen swap.
func (m Model) viewPalette() string {
	items := filterPaletteItems(buildPaletteItems(m), m.paletteQuery)

	width := m.width
	if width <= 0 {
		width = dashboardDefaultWidth
	}
	innerW := width - 6 // 6 = panel border + padding budget
	if innerW < 32 {
		innerW = 32
	}

	var b strings.Builder
	b.WriteString(ui.Render(theme.Header, "command palette") + "\n\n")
	b.WriteString(ui.Render(theme.Accent, "› ") +
		m.paletteQuery +
		ui.Render(theme.Accent, "█") + "\n\n")

	if len(items) == 0 {
		b.WriteString(ui.Render(theme.Dimmed, "no matching actions — try a shorter query"))
	} else {
		for i, it := range items {
			b.WriteString(renderPaletteRow(it, i == m.paletteCursor, innerW))
			b.WriteByte('\n')
		}
	}

	body := theme.Panel.
		Width(innerW + 2).
		Render(b.String())

	hint := ui.Render(theme.Dimmed, "↑/↓ select  •  enter run  •  esc close")
	return lipgloss.JoinVertical(lipgloss.Left, body, "", hint)
}

// renderPaletteRow formats one palette entry: caret on the
// selected row, label on the left, key shortcut on the right (or
// suppressed when there's no width budget for it).
func renderPaletteRow(it paletteAction, selected bool, width int) string {
	caret := "  "
	if selected {
		caret = ui.Render(theme.Accent, "▸ ")
	}
	keys := strings.Join(it.Keys, "/")
	keyHint := ui.Render(theme.KeyHint, keys)

	// Reserve the right column for the key hint and pad the
	// label to fill the rest. Falls back to label-only when the
	// pane is too narrow.
	const minPad = 2
	labelLen := len(it.Label)
	keysLen := len(keys)
	row := caret + it.Label
	avail := width - 2 // caret cells
	if avail-labelLen-keysLen >= minPad {
		pad := strings.Repeat(" ", avail-labelLen-keysLen)
		row = caret + it.Label + pad + keyHint
	}
	if selected {
		row = ui.Render(theme.Bold, row)
	}
	return row
}
